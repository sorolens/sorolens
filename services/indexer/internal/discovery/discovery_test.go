package discovery

import (
	"encoding/base64"
	"testing"
)

// Fixtures were generated with the Python stellar-sdk (v16.1.0): each
// envelope is signed with a deterministic key and the expected contract ID is
// computed by the SDK's own XDR serializer, so these tests check the
// hand-rolled decoder against an independent implementation. Account
// GCFIRY...YOJR signs every envelope; GCATS5...I55U is the second account
// (op source / fee-bump source / asset issuer).
var fixtures = []struct {
	name       string
	envelope   string
	contractID string
	source     string
	deployer   string
	isSAC      bool
	sac        *SACMetadata
}{
	{
		name:       "create_v2",
		envelope:   "AAAAAgAAAACKiOPddAnxlf1S2y08ul1yymcJvx2UEhvzdIgBtA9vXAAAAGQAAAAAAAAAZQAAAAEAAAAAAAAAAAAAAABqtc/XAAAAAAAAAAEAAAAAAAAAGAAAAAMAAAAAAAAAAAAAAACKiOPddAnxlf1S2y08ul1yymcJvx2UEhvzdIgBtA9vXAkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJAAAAAAcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHAAAAAAAAAAAAAAAAAAAAAbQPb1wAAABA5aONoYGOc+6cgzNer55szSaCepvoisKlMKivkWC20HBojwVgCjY2NkjZkN21ICcW7t8IQQQoLqd3BH3CER4fDA==",
		contractID: "CASLIQNFCMW3UXPTVA5KDQZK6NEBPK4K35XY2SEQ3D7JGRCOVLP2BKBK",
		source:     "GCFIRY65OQE7DFP5KLNS2PF2LVZMUZYJX4OZIEQ36N2IQANUB5XVYOJR",
		deployer:   "GCFIRY65OQE7DFP5KLNS2PF2LVZMUZYJX4OZIEQ36N2IQANUB5XVYOJR",
		isSAC:      false,
	},
	{
		name:       "op_source",
		envelope:   "AAAAAgAAAACKiOPddAnxlf1S2y08ul1yymcJvx2UEhvzdIgBtA9vXAAAAGQAAAAAAAAAZQAAAAIAAAABAAAAAAAAAAoAAAAAAAAAFAAAAAEAAAABAAAAYwAAAAAAAAAAAAAABQAAAAIAAAAAAAAAAQAAAAZkZXBsb3kAAAAAAAEAAAABAAAAAIE5dw6ofRdfVqNUZsNMfszLjYqRtO43ol32D1uPybOUAAAAGAAAAAMAAAAAAAAAAAAAAACBOXcOqH0XX1ajVGbDTH7My42KkbTuN6Jd9g9bj8mzlAkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJAAAAAAcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHAAAAAAAAAAAAAAAAAAAAAbQPb1wAAABAKFhA8afhlslY4xtZN0zjNwuVB+gaJNoDx5XRBJPltWcp2X3DuHH4CXL7f4ZSZTpkPDhlqTwk212W7BuzLkgDBw==",
		contractID: "CCQSCLV3N3LV27ZDXTRJG7FHWSSRMAQR47NF3LOVHRGW7FV6DFL5XR2V",
		source:     "GCATS5YOVB6ROX2WUNKGNQ2MP3GMXDMKSG2O4N5CLX3A6W4PZGZZI55U",
		deployer:   "GCATS5YOVB6ROX2WUNKGNQ2MP3GMXDMKSG2O4N5CLX3A6W4PZGZZI55U",
		isSAC:      false,
	},
	{
		name:       "fee_bump",
		envelope:   "AAAABQAAAACBOXcOqH0XX1ajVGbDTH7My42KkbTuN6Jd9g9bj8mzlAAAAAAAAAGQAAAAAgAAAACKiOPddAnxlf1S2y08ul1yymcJvx2UEhvzdIgBtA9vXAAAAGQAAAAAAAAAZQAAAAEAAAAAAAAAAAAAAABqtc/XAAAAAAAAAAEAAAAAAAAAGAAAAAMAAAAAAAAAAAAAAACKiOPddAnxlf1S2y08ul1yymcJvx2UEhvzdIgBtA9vXAkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJAAAAAAcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHAAAAAAAAAAAAAAAAAAAAAbQPb1wAAABA5aONoYGOc+6cgzNer55szSaCepvoisKlMKivkWC20HBojwVgCjY2NkjZkN21ICcW7t8IQQQoLqd3BH3CER4fDAAAAAAAAAABj8mzlAAAAEAuj8ej+KEdTGodux1vuIRoiQjSJY/pfrhKI4gi/JC5Ft2hQzaMGWbR5gdW9G5R/pi1Tb8WpwDB/O/h0hr36bgK",
		contractID: "CASLIQNFCMW3UXPTVA5KDQZK6NEBPK4K35XY2SEQ3D7JGRCOVLP2BKBK",
		source:     "GCFIRY65OQE7DFP5KLNS2PF2LVZMUZYJX4OZIEQ36N2IQANUB5XVYOJR",
		deployer:   "GCFIRY65OQE7DFP5KLNS2PF2LVZMUZYJX4OZIEQ36N2IQANUB5XVYOJR",
		isSAC:      false,
	},
	{
		name:       "invoke",
		envelope:   "AAAAAgAAAACKiOPddAnxlf1S2y08ul1yymcJvx2UEhvzdIgBtA9vXAAAAGQAAAAAAAAAZQAAAAEAAAAAAAAAAAAAAABqtc/XAAAAAAAAAAEAAAAAAAAAGAAAAAAAAAABJLRBpRMtul3zqDqhwyrzSBerit9vjUiQ2P6TRE6q36AAAAAFaGVsbG8AAAAAAAABAAAAAwAAAAEAAAAAAAAAAAAAAAG0D29cAAAAQP0wj7zSwY55/Ahlo8xbWNAiPJawWzM3G+QB67bP564OadKHK57t0E1ObpwQo4gJo6SzGyxkbvjxQuPXLtH6ago=",
		contractID: "",
		source:     "",
		deployer:   "",
		isSAC:      false,
	},
	{
		name:       "muxed_source",
		envelope:   "AAAAAgAAAQAAAAAAAAAAKoqI4910CfGV/VLbLTy6XXLKZwm/HZQSG/N0iAG0D29cAAAAZAAAAAAAAABlAAAAAQAAAAAAAAAAAAAAAGq1z9cAAAAAAAAAAQAAAAAAAAAYAAAAAwAAAAAAAAAAAAAAAIqI4910CfGV/VLbLTy6XXLKZwm/HZQSG/N0iAG0D29cCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkAAAAABwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcAAAAAAAAAAAAAAAAAAAABtA9vXAAAAEAkv3JBf3RazX2pPKNytK4gdWpApX+mOOpAoorIpV027klhXxxKF3MRIb/JuaJN/FxhW1+oJn7YklhXexy3B2UP",
		contractID: "CASLIQNFCMW3UXPTVA5KDQZK6NEBPK4K35XY2SEQ3D7JGRCOVLP2BKBK",
		source:     "GCFIRY65OQE7DFP5KLNS2PF2LVZMUZYJX4OZIEQ36N2IQANUB5XVYOJR",
		deployer:   "GCFIRY65OQE7DFP5KLNS2PF2LVZMUZYJX4OZIEQ36N2IQANUB5XVYOJR",
		isSAC:      false,
	},
	{
		name:       "from_asset",
		envelope:   "AAAAAgAAAACKiOPddAnxlf1S2y08ul1yymcJvx2UEhvzdIgBtA9vXAAAAGQAAAAAAAAAZQAAAAEAAAAAAAAAAAAAAABqtc/XAAAAAAAAAAEAAAAAAAAAGAAAAAEAAAABAAAAAVVTRAAAAAAAgTl3Dqh9F19Wo1Rmw0x+zMuNipG07jeiXfYPW4/Js5QAAAABAAAAAAAAAAAAAAABtA9vXAAAAEBmGxbSKcJ2C9cC/89XczCXUhmefHoKtOP+24b3GBBmMeiYgyJb/nQGzim1Ul/tE5ogKGQuL/ojv04gRFnKvW8O",
		contractID: "CDL5UKTHSI4MPDVKDOFM2MVN2PZN3G76NTHSE5BLEXINTTFAOZSHRXVU",
		source:     "GCFIRY65OQE7DFP5KLNS2PF2LVZMUZYJX4OZIEQ36N2IQANUB5XVYOJR",
		deployer:   "",
		isSAC:      true,
		sac: &SACMetadata{
			AssetCode: "USD",
			Issuer:    "GCATS5YOVB6ROX2WUNKGNQ2MP3GMXDMKSG2O4N5CLX3A6W4PZGZZI55U",
			Decimals:  7,
			Symbol:    "USD",
			Name:      "USD",
		},
	},
	{
		name:       "create_v1",
		envelope:   "AAAAAgAAAACKiOPddAnxlf1S2y08ul1yymcJvx2UEhvzdIgBtA9vXAAAAGQAAAAAAAAAZQAAAAEAAAAAAAAAAAAAAABqtc/XAAAAAAAAAAEAAAAAAAAAGAAAAAEAAAAAAAAAAAAAAACKiOPddAnxlf1S2y08ul1yymcJvx2UEhvzdIgBtA9vXAkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJAAAAAAcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHAAAAAAAAAAAAAAABtA9vXAAAAECB9aH16I4YQGsVpvdDsTt+3m3LR5YBu2ie6JprPqHPc5vMX1XFy7D3YAldrH8r+ObGg6Sy7n/3tNuPneyf9SUE",
		contractID: "CASLIQNFCMW3UXPTVA5KDQZK6NEBPK4K35XY2SEQ3D7JGRCOVLP2BKBK",
		source:     "GCFIRY65OQE7DFP5KLNS2PF2LVZMUZYJX4OZIEQ36N2IQANUB5XVYOJR",
		deployer:   "GCFIRY65OQE7DFP5KLNS2PF2LVZMUZYJX4OZIEQ36N2IQANUB5XVYOJR",
		isSAC:      false,
	},
}

func TestDeploymentsFixtures(t *testing.T) {
	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Deployments(tc.envelope, PassphraseTestnet)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.contractID == "" {
				if len(got) != 0 {
					t.Fatalf("want no deployments, got %+v", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("want 1 deployment, got %d", len(got))
			}
			d := got[0]
			if d.ContractID != tc.contractID {
				t.Errorf("contract id: want %s, got %s", tc.contractID, d.ContractID)
			}
			if d.Source != tc.source {
				t.Errorf("source: want %s, got %s", tc.source, d.Source)
			}
			if d.Deployer != tc.deployer {
				t.Errorf("deployer: want %q, got %q", tc.deployer, d.Deployer)
			}
			if d.IsSAC != tc.isSAC {
				t.Errorf("isSAC: want %v, got %v", tc.isSAC, d.IsSAC)
			}
			if tc.sac == nil {
				if d.SAC != nil {
					t.Errorf("sac metadata: want nil, got %+v", d.SAC)
				}
			} else {
				if d.SAC == nil {
					t.Fatalf("sac metadata: want %+v, got nil", tc.sac)
				}
				if *d.SAC != *tc.sac {
					t.Errorf("sac metadata: want %+v, got %+v", *tc.sac, *d.SAC)
				}
			}
		})
	}
}

func TestDeploymentsContractIDDependsOnNetwork(t *testing.T) {
	testnet, err := Deployments(fixtures[0].envelope, PassphraseTestnet)
	if err != nil {
		t.Fatal(err)
	}
	mainnet, err := Deployments(fixtures[0].envelope, PassphraseMainnet)
	if err != nil {
		t.Fatal(err)
	}
	if testnet[0].ContractID == mainnet[0].ContractID {
		t.Fatal("contract id must differ across networks")
	}
}

func TestDeploymentsMalformed(t *testing.T) {
	if _, err := Deployments("%%%not-base64", PassphraseTestnet); err == nil {
		t.Fatal("want error for invalid base64")
	}
	raw, err := base64.StdEncoding.DecodeString(fixtures[0].envelope)
	if err != nil {
		t.Fatal(err)
	}
	// Truncate inside the contract ID preimage: must error, never panic.
	for _, n := range []int{3, 20, 100, 150} {
		if n >= len(raw) {
			continue
		}
		trunc := base64.StdEncoding.EncodeToString(raw[:n])
		if _, err := Deployments(trunc, PassphraseTestnet); err == nil {
			t.Errorf("truncated at %d: want error", n)
		}
	}
}

func TestDeploymentsNativeAndAlphanum12(t *testing.T) {
	// Construct a synthetic envelope for Native asset SAC deployment
	// based on the valid from_asset fixture.
	raw, err := base64.StdEncoding.DecodeString(fixtures[5].envelope)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	// In from_asset:
	// offset 84 is opInvokeHostFunction (24)
	// offset 88 is hostFnCreateContract (1)
	// offset 92 is preimageFromAsset (1)
	// offset 96 is assetCreditAlphanum4 (1) -> 4 bytes code + 4 bytes keytype + 32 bytes key = 40 bytes
	// offset 136 is executable (1)
	// Let's replace the asset part with Native (assetNative = 0):
	var nativeEnv []byte
	nativeEnv = append(nativeEnv, raw[:96]...)
	// assetNative discriminant = 0 (4 bytes)
	nativeEnv = append(nativeEnv, 0, 0, 0, 0)
	// executable = 1
	nativeEnv = append(nativeEnv, 0, 0, 0, 1)
	// trailing signatures etc from offset 140
	nativeEnv = append(nativeEnv, raw[140:]...)

	encodedNative := base64.StdEncoding.EncodeToString(nativeEnv)
	deps, err := Deployments(encodedNative, PassphraseTestnet)
	if err != nil {
		t.Fatalf("native SAC deployments: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("want 1 deployment, got %d", len(deps))
	}
	d := deps[0]
	if !d.IsSAC {
		t.Errorf("want IsSAC=true, got %v", d.IsSAC)
	}
	if d.SAC == nil || d.SAC.AssetCode != "native" || d.SAC.Decimals != 7 {
		t.Errorf("want native SAC metadata, got %+v", d.SAC)
	}

	// Now construct an Alphanum12 SAC:
	var alpha12Env []byte
	alpha12Env = append(alpha12Env, raw[:96]...)
	// assetCreditAlphanum12 discriminant = 2 (4 bytes)
	alpha12Env = append(alpha12Env, 0, 0, 0, 2)
	// 12 bytes code: "LONGTOKEN12\x00"
	alpha12Env = append(alpha12Env, []byte("LONGTOKEN12\x00")...)
	// keytype 0 (4 bytes) + 32 bytes key from raw[104:140]
	alpha12Env = append(alpha12Env, raw[104:140]...)
	// executable = 1
	alpha12Env = append(alpha12Env, 0, 0, 0, 1)
	alpha12Env = append(alpha12Env, raw[140:]...)

	encoded12 := base64.StdEncoding.EncodeToString(alpha12Env)
	deps12, err := Deployments(encoded12, PassphraseTestnet)
	if err != nil {
		t.Fatalf("alphanum12 SAC deployments: %v", err)
	}
	if len(deps12) != 1 {
		t.Fatalf("want 1 deployment, got %d", len(deps12))
	}
	d12 := deps12[0]
	if !d12.IsSAC {
		t.Errorf("want IsSAC=true, got %v", d12.IsSAC)
	}
	if d12.SAC == nil || d12.SAC.AssetCode != "LONGTOKEN12" || d12.SAC.Decimals != 7 {
		t.Errorf("want LONGTOKEN12 SAC metadata, got %+v", d12.SAC)
	}
	if d12.SAC.Issuer != "GCATS5YOVB6ROX2WUNKGNQ2MP3GMXDMKSG2O4N5CLX3A6W4PZGZZI55U" {
		t.Errorf("want issuer GCATS..., got %s", d12.SAC.Issuer)
	}
}

func TestPassphrase(t *testing.T) {
	for network, want := range map[string]string{
		"":          PassphraseTestnet,
		"testnet":   PassphraseTestnet,
		"mainnet":   PassphraseMainnet,
		"futurenet": PassphraseFuturenet,
	} {
		if got, ok := Passphrase(network); !ok || got != want {
			t.Errorf("%q: want %q, got %q (ok=%v)", network, want, got, ok)
		}
	}
	if _, ok := Passphrase("moon"); ok {
		t.Error("unknown network must not resolve")
	}
}
