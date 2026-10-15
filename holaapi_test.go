package main

import (
	"encoding/json"
	"testing"
)

func TestZGetTunnelsBlocked(t *testing.T) {
	var res ZGetTunnelsResponse
	err := json.Unmarshal([]byte(`{"blocked":true,"permanent":false}`), &res)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !res.Blocked {
		t.Fatal("expected Blocked to be true")
	}
	if res.Permanent {
		t.Fatal("expected Permanent to be false")
	}
}

func TestZGetTunnelsPermanent(t *testing.T) {
	var res ZGetTunnelsResponse
	err := json.Unmarshal([]byte(`{"blocked":true,"permanent":true}`), &res)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !res.Blocked || !res.Permanent {
		t.Fatalf("expected blocked+permanent, got %+v", res)
	}
}

func TestZGetTunnelsNormal(t *testing.T) {
	var res ZGetTunnelsResponse
	err := json.Unmarshal([]byte(`{
		"agent_key": "secret",
		"ip_list": {"zagent1.hola.org": "1.2.3.4"},
		"port": {"direct": 22222, "peer": 22223, "hola": 22224, "trial": 22225, "trial_peer": 22226},
		"ztun": {"de": ["zagent1.hola.org:22222"]}
	}`), &res)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if res.Blocked || res.Permanent {
		t.Fatalf("expected no blocked flags, got %+v", res)
	}
	if res.Port.TrialPeer != 22226 {
		t.Fatalf("unexpected trial_peer port: %d", res.Port.TrialPeer)
	}
}

func TestCommonParams(t *testing.T) {
	params := commonParams("1.2.3")
	for _, k := range []string{"rmt_ver", "ext_ver", "browser", "product", "lccgi"} {
		if params.Get(k) == "" {
			t.Fatalf("commonParams missing %s", k)
		}
	}
	if params.Get("ext_ver") != "1.2.3" || params.Get("rmt_ver") != "1.2.3" {
		t.Fatal("ext_ver/rmt_ver mismatch")
	}
}
