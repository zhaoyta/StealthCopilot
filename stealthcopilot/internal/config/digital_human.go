package config

import "strings"

// DigitalHumanValidation reports startup gaps for the digital-human output mode.
type DigitalHumanValidation struct {
	MissingCredentials []string
	MissingStreamIDs   []string
	MissingDevices     []string
}

func (v DigitalHumanValidation) OK() bool {
	return len(v.MissingCredentials) == 0 && len(v.MissingStreamIDs) == 0 && len(v.MissingDevices) == 0
}

// ValidateDigitalHumanOutput 检查启动数字人输出所需配置是否完整，根据当前选择的 Provider 校验。
func (c *AppConfig) ValidateDigitalHumanOutput() DigitalHumanValidation {
	var v DigitalHumanValidation
	switch c.DigitalHumanProvider {
	case DigitalHumanProviderZego:
		if strings.TrimSpace(c.ZegoDigitalHumanAppID) == "" {
			v.MissingCredentials = append(v.MissingCredentials, "ZEGO AppID")
		}
		if strings.TrimSpace(c.ZegoServerSecret) == "" {
			v.MissingCredentials = append(v.MissingCredentials, "ZEGO ServerSecret")
		}
		if strings.TrimSpace(c.ZegoDigitalHumanID) == "" {
			v.MissingStreamIDs = append(v.MissingStreamIDs, "ZEGO 数字人 ID")
		}
	default: // DigitalHumanProviderSimli 及未知值均走 Simli 路径
		if strings.TrimSpace(c.SimliAPIKey) == "" {
			v.MissingCredentials = append(v.MissingCredentials, "Simli API Key")
		}
		if strings.TrimSpace(c.SimaliFaceID) == "" {
			v.MissingStreamIDs = append(v.MissingStreamIDs, "Simli Face ID")
		}
	}
	if strings.TrimSpace(c.VirtualMicName) == "" {
		v.MissingDevices = append(v.MissingDevices, "虚拟麦克风")
	}
	if strings.TrimSpace(c.VirtualCamName) == "" {
		v.MissingDevices = append(v.MissingDevices, "虚拟摄像头")
	}
	return v
}
