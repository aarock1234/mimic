package main

import (
	"encoding/json"
	"log/slog"
	"os"

	"github.com/aarock1234/mimic"
	_ "github.com/aarock1234/mimic/internal/logger"
	http "github.com/saucesteals/fhttp"
)

func main() {
	version := "137.0.0.0"
	if len(os.Args) > 1 {
		version = os.Args[1]
	}

	// edge uses the same chromium engine but with "Edg/{version}" in the user-agent
	spec, err := mimic.Chromium(mimic.BrandEdge, version)
	if err != nil {
		slog.Error("failed to create mimic spec", "error", err)
		return
	}

	transport, err := mimic.NewTransport(spec, mimic.PlatformWindows, // or mimic.PlatformMac, mimic.PlatformLinux
		mimic.WithBaseTransport(&http.Transport{Proxy: http.ProxyFromEnvironment}),
	)
	if err != nil {
		slog.Error("failed to create mimic transport", "error", err)
		return
	}

	client := &http.Client{Transport: transport}

	req, _ := http.NewRequest(http.MethodGet, "https://tls.peet.ws/api/clean", nil)

	req.Header.Add("rtt", "50")
	req.Header.Add("accept", "text/html,*/*")
	req.Header.Add("x-requested-with", "XMLHttpRequest")
	req.Header.Add("downlink", "3.9")
	req.Header.Add("ect", "4g")
	req.Header.Add("sec-fetch-site", "same-origin")
	req.Header.Add("sec-fetch-mode", "cors")
	req.Header.Add("sec-fetch-dest", "empty")
	req.Header.Add("accept-encoding", "gzip, deflate, br")
	req.Header.Add("accept-language", "en,en_US;q=0.9")
	// mimic automatically sets: user-agent, sec-ch-ua, sec-ch-ua-mobile, sec-ch-ua-platform

	res, err := client.Do(req)
	if err != nil {
		slog.Error("failed to make request", "error", err)
		return
	}

	defer res.Body.Close()

	var response PeetCleanResponse
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		slog.Error("failed to decode peet clean response", "error", err)
		return
	}

	slog.Info("request", "method", req.Method, "url", req.URL)

	for key, value := range req.Header {
		if key == http.HeaderOrderKey || key == http.PHeaderOrderKey {
			continue
		}

		slog.Info("request header", "key", key, "value", value[0])
	}

	slog.Info("JA3", "value", response.Ja3)
	slog.Info("JA3 Hash", "value", response.Ja3Hash)
	slog.Info("JA4", "value", response.Ja4)
	slog.Info("JA4-R", "value", response.Ja4R)
	slog.Info("Akamai", "value", response.Akamai)
	slog.Info("Akamai Hash", "value", response.AkamaiHash)
	slog.Info("Peetprint", "value", response.Peetprint)
	slog.Info("Peetprint Hash", "value", response.PeetprintHash)
}

type PeetCleanResponse struct {
	Ja3           string `json:"ja3"`
	Ja3Hash       string `json:"ja3_hash"`
	Ja4           string `json:"ja4"`
	Ja4R          string `json:"ja4_r"`
	Akamai        string `json:"akamai"`
	AkamaiHash    string `json:"akamai_hash"`
	Peetprint     string `json:"peetprint"`
	PeetprintHash string `json:"peetprint_hash"`
}
