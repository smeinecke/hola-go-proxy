package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
	"github.com/campoy/unique"
	"github.com/cenkalti/backoff/v4"
	"github.com/google/uuid"
)

const EXT_BROWSER = "chrome"
const PRODUCT = "cws"
const CCGI_URL = "https://client.hola.org/client_cgi/"
const VPN_COUNTRIES_URL = CCGI_URL + "vpn_countries.json"
const RULES_GET_VPN_URL = CCGI_URL + "rules_get_vpn.json"
const BG_INIT_URL = CCGI_URL + "background_init"
const ZGETTUNNELS_URL = CCGI_URL + "zgettunnels"
const AGENT_SUFFIX = ".hola.org"

// Mimic a real Chrome extension request as closely as possible.
const EXT_ORIGIN = "chrome-extension://gkojfkhlekighikafcpjkiklfbnlmeio"
const ACCEPT_LANGUAGE = "de-DE,de;q=0.9,en-US;q=0.8,en;q=0.7"

var HEADER_ORDER = []string{
	"host",
	"connection",
	"content-length",
	"content-type",
	"accept",
	"origin",
	"user-agent",
	"accept-encoding",
	"accept-language",
}

var FALLBACK_CONF_URLS = []string{
	"https://www.dropbox.com/s/jemizcvpmf2qb9v/cloud_failover.conf?dl=1",
	"https://vdkd6nz8qr.s3.amazonaws.com/cloud_failover.conf",
}

var LOGIN_TEMPLATE = template.Must(template.New("LOGIN_TEMPLATE").Parse("user-uuid-{{.uuid}}-is_prem-{{.prem}}"))
var TemporaryBanError = errors.New("temporary ban detected")
var PermanentBanError = errors.New("permanent ban detected")
var EmptyResponseError = errors.New("empty response")

var userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36"

func SetUserAgent(ua string) {
	userAgent = ua
}

func GetUserAgent() string {
	return userAgent
}

type CountryList []string

type BgInitResponse struct {
	Ver       string `json:"ver"`
	Key       int64  `json:"key"`
	Country   string `json:"country"`
	Blocked   bool   `json:"blocked,omitempty"`
	Permanent bool   `json:"permanent,omitempty"`
}

type PortMap struct {
	Direct    uint16 `json:"direct"`
	Hola      uint16 `json:"hola"`
	Peer      uint16 `json:"peer"`
	Trial     uint16 `json:"trial"`
	TrialPeer uint16 `json:"trial_peer"`
}

type ZGetTunnelsResponse struct {
	AgentKey   string              `json:"agent_key"`
	AgentTypes map[string]string   `json:"agent_types"`
	IPList     map[string]string   `json:"ip_list"`
	Port       PortMap             `json:"port"`
	Protocol   map[string]string   `json:"protocol"`
	Vendor     map[string]string   `json:"vendor"`
	Ztun       map[string][]string `json:"ztun"`
	Blocked    bool                `json:"blocked,omitempty"`
	Permanent  bool                `json:"permanent,omitempty"`
}

type FallbackAgent struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
	Port uint16 `json:"port"`
}

type fallbackConfResponse struct {
	Agents    []FallbackAgent `json:"agents"`
	UpdatedAt int64           `json:"updated_ts"`
	TTL       int64           `json:"ttl_ms"`
}

type FallbackConfig struct {
	Agents    []FallbackAgent
	UpdatedAt time.Time
	TTL       time.Duration
}

func (c *FallbackConfig) UnmarshalJSON(data []byte) error {
	r := fallbackConfResponse{}
	err := json.Unmarshal(data, &r)
	if err != nil {
		return err
	}
	c.Agents = r.Agents
	c.UpdatedAt = time.Unix(r.UpdatedAt/1000, (r.UpdatedAt%1000)*1000000)
	c.TTL = time.Duration(r.TTL * 1000000)
	return nil
}

func (c *FallbackConfig) Expired() bool {
	return time.Now().After(c.UpdatedAt.Add(c.TTL))
}

func (c *FallbackConfig) ShuffleAgents() {
	rand.New(RandomSource).Shuffle(len(c.Agents), func(i, j int) {
		c.Agents[i], c.Agents[j] = c.Agents[j], c.Agents[i]
	})
}

func (c *FallbackConfig) Clone() *FallbackConfig {
	return &FallbackConfig{
		Agents:    append([]FallbackAgent(nil), c.Agents...),
		UpdatedAt: c.UpdatedAt,
		TTL:       c.TTL,
	}
}

func (a *FallbackAgent) ToProxy() *url.URL {
	return &url.URL{
		Scheme: "https",
		Host: net.JoinHostPort(a.Name+AGENT_SUFFIX,
			fmt.Sprintf("%d", a.Port)),
	}
}

func (a *FallbackAgent) Hostname() string {
	return a.Name + AGENT_SUFFIX
}

func (a *FallbackAgent) NetAddr() string {
	return net.JoinHostPort(a.IP, fmt.Sprintf("%d", a.Port))
}

var cclient = cycletls.Init()

func commonParams(extVer string) url.Values {
	params := make(url.Values)
	params.Add("rmt_ver", extVer)
	params.Add("ext_ver", extVer)
	params.Add("browser", EXT_BROWSER)
	params.Add("product", PRODUCT)
	params.Add("lccgi", "1")
	return params
}

func do_req(ctx context.Context, proxyURL, method, reqURL string, query, data url.Values) ([]byte, error) {
	if method == "" {
		method = "GET"
	}
	timeout := 35
	if dl, ok := ctx.Deadline(); ok {
		timeout = int(math.Ceil(time.Until(dl).Seconds()))
		if timeout < 1 {
			timeout = 1
		}
	}
	if query != nil {
		sep := "?"
		if strings.Contains(reqURL, "?") {
			sep = "&"
		}
		reqURL += sep + query.Encode()
	}
	opts := cycletls.Options{
		URL:                   reqURL,
		UserAgent:             userAgent,
		Proxy:                 proxyURL,
		Timeout:               timeout,
		DisableRedirect:       true,
		EnableConnectionReuse: true,
		HeaderOrder:           HEADER_ORDER,
		Headers: map[string]string{
			"Origin":          EXT_ORIGIN,
			"Accept-Language": ACCEPT_LANGUAGE,
		},
	}
	if data != nil {
		opts.Body = data.Encode()
		opts.Headers["Content-Type"] = "application/x-www-form-urlencoded"
	}
	resp, err := cclient.Do(reqURL, opts, method)
	if err != nil {
		return nil, err
	}
	if resp.Status < 200 || resp.Status > 204 {
		return nil, fmt.Errorf("bad HTTP response: %d", resp.Status)
	}
	return []byte(resp.Body), nil
}

func VPNCountries(ctx context.Context, proxyURL, extVer string) (res CountryList, err error) {
	params := commonParams(extVer)
	data, err := do_req(ctx, proxyURL, "", VPN_COUNTRIES_URL, params, nil)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &res)
	for _, a := range res {
		if a == "uk" {
			res = append(res, "gb")
		}
	}
	less := func(i, j int) bool { return res[i] < res[j] }
	unique.Slice(&res, less)
	return
}

func background_init(ctx context.Context, proxyURL, extVer, user_uuid string) (res BgInitResponse, reterr error) {
	post_data := make(url.Values)
	post_data.Add("login", "1")
	post_data.Add("flags", "0")
	post_data.Add("ver", extVer)
	qs := commonParams(extVer)
	qs.Add("uuid", user_uuid)
	resp, err := do_req(ctx, proxyURL, "POST", BG_INIT_URL, qs, post_data)
	if err != nil {
		reterr = err
		return
	}

	reterr = json.Unmarshal(resp, &res)
	if reterr == nil && res.Blocked {
		if res.Permanent {
			reterr = PermanentBanError
		} else {
			reterr = TemporaryBanError
		}
	}
	return
}

func validateSession(ctx context.Context, proxyURL, extVer, user_uuid string, session_key int64) (bool, error) {
	params := commonParams(extVer)
	params.Add("uuid", user_uuid)
	params.Add("session_key", strconv.FormatInt(session_key, 10))
	data, err := do_req(ctx, proxyURL, "", RULES_GET_VPN_URL, params, nil)
	if err != nil {
		return false, err
	}
	var obj struct {
		Blocked bool `json:"blocked"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		// A non-object truthy response still means the session was accepted.
		return true, nil
	}
	return !obj.Blocked, nil
}

func zgettunnels(ctx context.Context,
	proxyURL string,
	user_uuid string,
	session_key int64,
	extVer string,
	country string,
	proxy_type string,
	limit uint) (res *ZGetTunnelsResponse, reterr error) {
	var tunnels ZGetTunnelsResponse
	params := commonParams(extVer)
	if proxy_type == "lum" {
		params.Add("country", country+".pool_lum_"+country+"_shared")
	} else if proxy_type == "virt" { // seems to be for brazil and japan only
		params.Add("country", country+".pool_virt_pool_"+country)
	} else if proxy_type == "peer" {
		//params.Add("country", country + ".peer")
		params.Add("country", country)
	} else if proxy_type == "pool" {
		params.Add("country", country+".pool")
	} else { // direct or skip
		params.Add("country", country)
	}
	params.Add("limit", strconv.FormatInt(int64(limit), 10))
	params.Add("ping_id", strconv.FormatFloat(rand.New(RandomSource).Float64(), 'f', -1, 64))
	params.Add("uuid", user_uuid)
	params.Add("session_key", strconv.FormatInt(session_key, 10))
	params.Add("is_premium", "0")
	data, err := do_req(ctx, proxyURL, "POST", ZGETTUNNELS_URL, params, nil)
	if err != nil {
		reterr = err
		return
	}
	err = json.Unmarshal(data, &tunnels)
	if err != nil {
		return nil, fmt.Errorf("unable to unmashal zgettunnels response: %w", err)
	}
	if tunnels.Blocked {
		if tunnels.Permanent {
			return nil, PermanentBanError
		}
		return nil, TemporaryBanError
	}
	if len(tunnels.IPList) == 0 {
		return nil, EmptyResponseError
	}
	res = &tunnels
	return
}

func fetchFallbackConfig(ctx context.Context) (*FallbackConfig, error) {
	fallbackConfURL := FALLBACK_CONF_URLS[rand.New(RandomSource).Intn(len(FALLBACK_CONF_URLS))]
	confRaw, err := do_req(ctx, "", "", fallbackConfURL, nil, nil)
	if err != nil {
		return nil, err
	}

	l := len(confRaw)
	if l < 4 {
		return nil, errors.New("bad response length from fallback conf URL")
	}

	buf := &bytes.Buffer{}
	buf.Grow(l)
	buf.Write(confRaw[l-3:])
	buf.Write(confRaw[:l-3])

	b64dec := base64.NewDecoder(base64.RawStdEncoding, buf)
	jdec := json.NewDecoder(b64dec)
	fbc := &FallbackConfig{}

	err = jdec.Decode(fbc)
	if err != nil {
		return nil, err
	}

	if fbc.Expired() {
		return nil, errors.New("fetched expired fallback config")
	}

	fbc.ShuffleAgents()
	return fbc, nil
}

var (
	fbcMux    sync.Mutex
	cachedFBC *FallbackConfig
)

func GetFallbackProxies(ctx context.Context) (*FallbackConfig, error) {
	fbcMux.Lock()
	defer fbcMux.Unlock()

	var (
		fbc *FallbackConfig
		err error
	)

	if cachedFBC == nil || cachedFBC.Expired() {
		fbc, err = fetchFallbackConfig(ctx)
		if err != nil {
			return nil, err
		}
		cachedFBC = fbc
	} else {
		fbc = cachedFBC
	}

	return fbc.Clone(), nil
}

func Tunnels(ctx context.Context,
	logger *CondLogger,
	proxyURL string,
	extVer string,
	country string,
	proxy_type string,
	limit uint,
	timeout time.Duration,
	backoffInitial time.Duration,
	backoffDeadline time.Duration,
	sessionFile string,
) (res *ZGetTunnelsResponse, user_uuid string, reterr error) {
	var sessionKey int64
	sess, err := loadSession(sessionFile)
	if err != nil {
		logger.Warning("Could not load session file %s: %v", sessionFile, err)
	}
	if sess != nil {
		ctx1, cancel := context.WithTimeout(ctx, timeout)
		valid, verr := validateSession(ctx1, proxyURL, extVer, sess.UUID, sess.SessionKey)
		cancel()
		if verr == nil && valid {
			user_uuid, sessionKey = sess.UUID, sess.SessionKey
			logger.Info("Reusing session from %s", sessionFile)
		} else {
			logger.Warning("Stored session is invalid (%v), registering a new one", verr)
		}
	}
	if user_uuid == "" {
		u := uuid.New()
		user_uuid = hex.EncodeToString(u[:])
		ctx1, cancel := context.WithTimeout(ctx, timeout)
		initres, err := background_init(ctx1, proxyURL, extVer, user_uuid)
		cancel()
		if err != nil {
			reterr = err
			return
		}
		sessionKey = initres.Key
		sess = &Session{UUID: user_uuid, SessionKey: sessionKey}
		if err := sess.save(sessionFile); err != nil {
			logger.Warning("Could not save session file %s: %v", sessionFile, err)
		}
	}
	var bo backoff.BackOff = &backoff.ExponentialBackOff{
		InitialInterval:     backoffInitial,
		RandomizationFactor: 0.5,
		Multiplier:          1.5,
		MaxInterval:         10 * time.Minute,
		MaxElapsedTime:      backoffDeadline,
		Stop:                backoff.Stop,
		Clock:               backoff.SystemClock,
	}
	bo = backoff.WithContext(bo, ctx)
	err = backoff.RetryNotify(func() error {
		ctx1, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		res, reterr = zgettunnels(ctx1, proxyURL, user_uuid, sessionKey, extVer, country, proxy_type, limit)
		if errors.Is(reterr, PermanentBanError) {
			return backoff.Permanent(reterr)
		}
		return reterr
	}, bo, func(err error, dur time.Duration) {
		logger.Info("zgettunnels error: %v; will retry after %v", err, dur.Truncate(time.Millisecond))
	})
	if err != nil {
		logger.Error("All attempts failed: %v", err)
		return nil, "", err
	}
	return
}

var hideSNI bool

func SetHideSNI(hide bool) {
	hideSNI = hide
}

func EnsureTransaction(ctx context.Context, getFBTimeout time.Duration, baseProxy string, txn func(context.Context, string) bool) (bool, error) {
	if txn(ctx, baseProxy) {
		return true, nil
	}

	// Fallback needed
	getFBCtx, cancel := context.WithTimeout(ctx, getFBTimeout)
	defer cancel()
	fbc, err := GetFallbackProxies(getFBCtx)
	if err != nil {
		return false, err
	}

	for _, agent := range fbc.Agents {
		if txn(ctx, agent.ToProxy().String()) {
			return true, nil
		}
	}

	return false, nil
}

func TemplateLogin(user_uuid string) string {
	var b strings.Builder
	LOGIN_TEMPLATE.Execute(&b, map[string]string{
		"uuid": user_uuid,
		"prem": "0",
	})
	return b.String()
}
