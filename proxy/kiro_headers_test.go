package proxy

import (
	"kiro-go/config"
	"strings"
	"testing"
)

func TestBuildStreamingHeaderValuesAlignsWithKiroIDEFormat(t *testing.T) {
	account := &config.Account{MachineId: "machine-123"}
	values := buildStreamingHeaderValues(account, "q.us-east-1.amazonaws.com")

	if values.Host != "q.us-east-1.amazonaws.com" {
		t.Fatalf("expected host to be preserved, got %q", values.Host)
	}
	if !strings.Contains(values.UserAgent, "aws-sdk-js/1.0.34") {
		t.Fatalf("expected streaming sdk version in user agent, got %q", values.UserAgent)
	}
	if !strings.Contains(values.UserAgent, "api/codewhispererstreaming#1.0.34") {
		t.Fatalf("expected streaming API marker in user agent, got %q", values.UserAgent)
	}
	if !strings.Contains(values.UserAgent, "KiroIDE-0.11.107-machine-123") {
		t.Fatalf("expected kiro version and machine id in user agent, got %q", values.UserAgent)
	}
	if !strings.Contains(values.AmzUserAgent, "aws-sdk-js/1.0.34 KiroIDE-0.11.107-machine-123") {
		t.Fatalf("expected x-amz-user-agent to include version and machine id, got %q", values.AmzUserAgent)
	}
}

func TestBuildRuntimeHeaderValuesUsesRuntimeAPIFormat(t *testing.T) {
	account := &config.Account{MachineId: "machine-456"}
	values := buildRuntimeHeaderValues(account, "codewhisperer.us-east-1.amazonaws.com")

	if !strings.Contains(values.UserAgent, "aws-sdk-js/1.0.0") {
		t.Fatalf("expected runtime sdk version in user agent, got %q", values.UserAgent)
	}
	if !strings.Contains(values.UserAgent, "api/codewhispererruntime#1.0.0") {
		t.Fatalf("expected runtime API marker in user agent, got %q", values.UserAgent)
	}
	if !strings.Contains(values.UserAgent, "m/N,E") {
		t.Fatalf("expected runtime mode marker in user agent, got %q", values.UserAgent)
	}
}

func TestBuildStreamingHeaderValuesUsesKiroCLIFingerprint(t *testing.T) {
	account := &config.Account{ClientMode: config.ClientModeKiroCLI}
	values := buildStreamingHeaderValues(account, "q.us-east-1.amazonaws.com")

	if !strings.Contains(values.UserAgent, "aws-sdk-rust/1.3.14") {
		t.Fatalf("expected rust sdk marker in cli user agent, got %q", values.UserAgent)
	}
	if !strings.Contains(values.UserAgent, "app/AmazonQ-For-CLI") {
		t.Fatalf("expected AmazonQ CLI app marker, got %q", values.UserAgent)
	}
	if strings.Contains(values.UserAgent, "KiroIDE") {
		t.Fatalf("did not expect KiroIDE marker in cli mode user agent, got %q", values.UserAgent)
	}
	if !strings.Contains(values.AmzUserAgent, "api/codewhispererstreaming/0.1.14474") {
		t.Fatalf("expected cli streaming api marker in x-amz-user-agent, got %q", values.AmzUserAgent)
	}
}
