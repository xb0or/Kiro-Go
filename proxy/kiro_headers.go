package proxy

import (
	"kiro-go/config"
	"net/http"
)

const (
	kiroStreamingSDKVersion = "1.0.34"
	kiroRuntimeSDKVersion   = "1.0.0"
)

type kiroHeaderValues struct {
	UserAgent    string
	AmzUserAgent string
	Host         string
}

func buildStreamingHeaderValues(account *config.Account, host string) kiroHeaderValues {
	return buildKiroHeaderValues(account, host, "codewhispererstreaming", kiroStreamingSDKVersion, "m/E")
}

func buildRuntimeHeaderValues(account *config.Account, host string) kiroHeaderValues {
	return buildKiroHeaderValues(account, host, "codewhispererruntime", kiroRuntimeSDKVersion, "m/N,E")
}

func buildKiroHeaderValues(account *config.Account, host, apiName, sdkVersion, modeMarker string) kiroHeaderValues {
	clientCfg := config.GetKiroClientConfig()
	clientMode := config.EffectiveClientMode(account)
	machineID := ""
	if account != nil {
		machineID = account.MachineId
	}

	var userAgent, amzUserAgent string
	if apiName == "codewhispererstreaming" {
		userAgent = clientCfg.StreamingUserAgent(machineID, clientMode)
		amzUserAgent = clientCfg.StreamingAmzUserAgent(machineID, clientMode)
	} else {
		userAgent = clientCfg.RuntimeUserAgent(machineID, clientMode)
		amzUserAgent = clientCfg.RuntimeAmzUserAgent(machineID, clientMode)
	}

	return kiroHeaderValues{
		UserAgent:    userAgent,
		AmzUserAgent: amzUserAgent,
		Host:         host,
	}
}

func applyKiroBaseHeaders(req *http.Request, account *config.Account, values kiroHeaderValues) {
	if account != nil && account.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+account.AccessToken)
	}
	req.Header.Set("User-Agent", values.UserAgent)
	req.Header.Set("x-amz-user-agent", values.AmzUserAgent)
	if config.EffectiveClientMode(account).IsCLI() {
		req.Header.Set("x-amzn-codewhisperer-optout", "false")
	} else {
		req.Header.Set("x-amzn-codewhisperer-optout", "true")
	}
	if values.Host != "" {
		req.Host = values.Host
	}
}
