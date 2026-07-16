package challenge

import "testing"

func TestImageRegistryHost(t *testing.T) {
	cases := map[string]string{
		"registry.example.com:5000/team/lab@sha256:abc": "registry.example.com:5000",
		"localhost/team/lab@sha256:abc":                 "localhost",
		"team/lab@sha256:abc":                           "docker.io",
		"lab@sha256:abc":                                "docker.io",
	}
	for input, expected := range cases {
		if actual := imageRegistryHost(input); actual != expected {
			t.Fatalf("imageRegistryHost(%q)=%q, want %q", input, actual, expected)
		}
	}
}

func TestValidateRuntime(t *testing.T) {
	digest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	valid := &RuntimeMutation{ImageRef: "example/ctf@" + digest, ImageDigest: digest, InternalPort: 8080, Protocol: "http", FlagFormat: "uuid", CPUMilli: 250, MemoryMB: 128, PIDsLimit: 64, DiskMB: 64, TTLSeconds: 7200, MaxTTLSeconds: 14400, ReadOnlyRootFS: true, Environment: map[string]string{"APP_MODE": "challenge"}}
	if err := validateRuntime(valid); err != nil {
		t.Fatalf("valid runtime rejected: %v", err)
	}
	local := *valid
	local.ImageRef = "ctf-upload:latest"
	local.ImageDigest = ""
	if err := validateRuntime(&local); err != nil {
		t.Fatalf("local tagged image rejected: %v", err)
	}
	defaults := local
	defaults.Protocol = ""
	defaults.FlagFormat = ""
	if err := validateRuntime(&defaults); err != nil {
		t.Fatalf("runtime defaults rejected: %v", err)
	}
	if defaults.Protocol != "tcp" || defaults.FlagFormat != "standard" {
		t.Fatalf("unexpected runtime defaults: protocol=%q flagFormat=%q", defaults.Protocol, defaults.FlagFormat)
	}
	invalid := *valid
	invalid.Environment = map[string]string{"ASAMU_FLAG": "override"}
	if err := validateRuntime(&invalid); err == nil {
		t.Fatal("reserved environment variable should be rejected")
	}
	invalid = *valid
	invalid.ImageDigest = "sha256:short"
	if err := validateRuntime(&invalid); err == nil {
		t.Fatal("invalid digest should be rejected")
	}
	invalid = *valid
	invalid.ImageRef = "ctf upload:latest"
	invalid.ImageDigest = ""
	if err := validateRuntime(&invalid); err == nil {
		t.Fatal("image reference containing whitespace should be rejected")
	}
	invalid = *valid
	invalid.FlagFormat = "timestamp"
	if err := validateRuntime(&invalid); err == nil {
		t.Fatal("unknown dynamic Flag format should be rejected")
	}
}

func TestNormalizeMutationValidatesServerSideFields(t *testing.T) {
	valid := Mutation{Title: "  Example  ", CategoryKey: "web", BaseScore: 300, MinimumScore: 100, MaximumScore: 500, ScoreMode: "dynamic", Visibility: "public"}
	if err := normalizeMutation(&valid); err != nil {
		t.Fatalf("valid challenge rejected: %v", err)
	}
	if valid.Title != "Example" || valid.DynamicDecay != 50 {
		t.Fatalf("challenge defaults were not normalized: %#v", valid)
	}
	invalid := valid
	invalid.BaseScore = 50
	if err := normalizeMutation(&invalid); err == nil {
		t.Fatal("base score below minimum should be rejected")
	}
	invalid = valid
	invalid.Visibility = "secret"
	if err := normalizeMutation(&invalid); err == nil {
		t.Fatal("unknown visibility should be rejected")
	}
}
