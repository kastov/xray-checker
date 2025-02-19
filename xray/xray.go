package xray

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"text/template"
	"xray-checker/checker"
	"xray-checker/config"
	"xray-checker/models"
	"xray-checker/runner"
	"xray-checker/web"
)

type TemplateData struct {
	Proxies      []*models.ProxyConfig
	StartPort    int
	XrayLogLevel string
}

func generateConfig(proxies []*models.ProxyConfig, startPort int, xrayLogLevel string) ([]byte, error) {
	data := TemplateData{
		Proxies:      proxies,
		StartPort:    startPort,
		XrayLogLevel: xrayLogLevel,
	}

	tmpl, err := template.New("xray.json.tmpl").
		Funcs(template.FuncMap{
			"add": func(a, b int) int { return a + b },
		}).
		ParseFS(templates, "templates/xray.json.tmpl")
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	var jsonCheck interface{}
	if err := json.Unmarshal(buf.Bytes(), &jsonCheck); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func saveConfig(config []byte, filename string) error {
	if err := os.WriteFile(filename, config, 0644); err != nil {
		return fmt.Errorf("error writing config file: %v", err)
	}
	return nil
}

func PrepareProxyConfigs(proxies []*models.ProxyConfig) {
	for i := range proxies {
		proxies[i].Index = i
	}
}

func GenerateAndSaveConfig(proxies []*models.ProxyConfig, startPort int, filename string, xrayLogLevel string) error {
	configBytes, err := generateConfig(proxies, startPort, xrayLogLevel)
	if err != nil {
		return fmt.Errorf("error generating config: %v", err)
	}

	if err := saveConfig(configBytes, filename); err != nil {
		return fmt.Errorf("error saving config: %v", err)
	}

	return nil
}

func UpdateConfiguration(newConfigs []*models.ProxyConfig, currentConfigs *[]*models.ProxyConfig,
	xrayRunner *runner.XrayRunner, proxyChecker *checker.ProxyChecker) error {

	log.Println("Found changes in subscription, updating configuration...")

	PrepareProxyConfigs(newConfigs)

	configFile := "xray_config.json"
	if err := GenerateAndSaveConfig(newConfigs, config.CLIConfig.Xray.StartPort, configFile, config.CLIConfig.Xray.LogLevel); err != nil {
		return fmt.Errorf("error generating new Xray config: %v", err)
	}

	if err := xrayRunner.Stop(); err != nil {
		return fmt.Errorf("error stopping Xray: %v", err)
	}

	if err := xrayRunner.Start(); err != nil {
		return fmt.Errorf("error starting Xray with new config: %v", err)
	}

	proxyChecker.UpdateProxies(newConfigs)

	*currentConfigs = newConfigs

	web.RegisterConfigEndpoints(newConfigs, config.CLIConfig.Xray.StartPort)

	log.Println("Configuration updated successfully")
	return nil
}

func IsConfigsEqual(old, new []*models.ProxyConfig) bool {
	if len(old) != len(new) {
		return false
	}

	oldMap := make(map[string]bool)
	for _, cfg := range old {
		key := fmt.Sprintf("%s:%s:%d:%s", cfg.Protocol, cfg.Server, cfg.Port, cfg.Name)
		oldMap[key] = true
	}

	for _, cfg := range new {
		key := fmt.Sprintf("%s:%s:%d:%s", cfg.Protocol, cfg.Server, cfg.Port, cfg.Name)
		if !oldMap[key] {
			return false
		}
	}

	return true
}
