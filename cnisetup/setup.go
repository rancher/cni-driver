package cnisetup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	log "github.com/Sirupsen/logrus"
	"github.com/rancher/cni-driver/utils"
	"github.com/rancher/go-rancher-metadata/metadata"
)

const (
	metadataURLTemplate = "http://%v/2016-07-29"
	cniDir              = "/opt/cni-driver/%s.d"
	binDir              = "/opt/cni-driver/bin"

	// DefaultMetadataAddress specifies the default value to use if nothing is specified
	DefaultMetadataAddress = "169.254.169.250"
)

// Do ...
func Do(metadataAddress string) error {
	var err error
	isSuccess := false
	metadataURL := fmt.Sprintf(metadataURLTemplate, metadataAddress)

	log.Infof("Waiting for metadata")
	mc, err := metadata.NewClientAndWait(metadataURL)
	if err != nil {
		return err
	}

	networks, err := mc.GetNetworks()
	if err != nil {
		return err
	}

	host, err := mc.GetSelfHost()
	if err != nil {
		return err
	}

	for _, network := range networks {
		if network.EnvironmentUUID != host.EnvironmentUUID {
			log.Debugf("cnisetup: %v is not local to this environment", network.UUID)
			continue
		}
		conf, ok := network.Metadata["cniConfig"].(map[string]interface{})
		if !ok {
			log.Debugf("cnisetup: network: %v is not a CNI network", network)
			continue
		}

		var cniBinaryName string
		for _, file := range conf {
			file = utils.UpdateCNIConfigByKeywords(file, host)
			props, _ := file.(map[string]interface{})
			cniBinaryName, _ = props["type"].(string)
		}

		log.Infof("cnisetup: setting up CNI config file")
		if err := setupCNIConfig(network, host); err != nil {
			return fmt.Errorf("cnisetup: failed to setup cni config: %v", err)
		}

		log.Infof("cnisetup: setting up CNI wrapper binary file")
		if err := setupCNIBinary(host, cniBinaryName); err != nil {
			return fmt.Errorf("cnisetup: failed to setup cni binary: %v", err)
		}
		isSuccess = true
	}

	if isSuccess {
		log.Infof("cnisetup: success")
	} else {
		if err != nil {
			log.Errorf("cnisetup: error: %v", err)
		} else {
			err = fmt.Errorf("cnisetup: no setup happened")
		}
		return err
	}

	return nil
}

func setupCNIConfig(network metadata.Network, host metadata.Host) error {
	cniConf, _ := network.Metadata["cniConfig"].(map[string]interface{})
	confDir := fmt.Sprintf(cniDir, network.Name)
	if err := os.MkdirAll(confDir, 0700); err != nil {
		return err
	}

	var lastErr error
	for file, config := range cniConf {
		config = utils.UpdateCNIConfigByKeywords(config, host)
		p := filepath.Join(confDir, file)
		content, err := json.Marshal(config)
		if err != nil {
			lastErr = err
			continue
		}

		out := &bytes.Buffer{}
		if err := json.Indent(out, content, "", "  "); err != nil {
			lastErr = err
			continue
		}

		log.Debugf("Writing %s: %s", p, out)
		if err := ioutil.WriteFile(p, out.Bytes(), 0600); err != nil {
			lastErr = err
		}
	}

	if network.Default {
		managedDir := fmt.Sprintf(cniDir, "managed")
		managedDirTest, err := os.Stat(managedDir)
		configDirTest, err1 := os.Stat(confDir)
		if !(err == nil && err1 == nil && os.SameFile(managedDirTest, configDirTest)) {
			os.Remove(managedDir)
			if err := os.Symlink(network.Name+".d", managedDir); err != nil {
				lastErr = err
			}
		}
	}

	return lastErr
}

func setupCNIBinary(host metadata.Host, name string) error {
	script := `#!/bin/sh
exec /usr/bin/nsenter -m -u -i -n -p -t %d -- $0 "$@"
`

	os.MkdirAll(binDir, 0700)

	ppid := os.Getppid()
	log.Debugf("cnisetup: ppid: %v", ppid)

	ptmp := filepath.Join(binDir, name+".tmp")
	p := filepath.Join(binDir, name)
	content := []byte(fmt.Sprintf(script, ppid))
	log.Debugf("Writing %s:\n%s", p, content)
	if err := ioutil.WriteFile(ptmp, content, 0700); err != nil {
		log.Errorf("cnisetup: error creating cni binary file: %v", err)
		return err
	}

	// Workaround for docker bug where the mount gets
	// created as a directory instead of file
	fileInfo, err := os.Stat(p)
	if err == nil && fileInfo.IsDir() {
		log.Infof("%s is a dir, removing it", p)
		if err = os.Remove(p); err != nil {
			log.Errorf("cnisetup: error removing incorrect directory created with binary name: %v", err)
			return err
		}
	}

	if err := os.Rename(ptmp, p); err != nil {
		log.Errorf("cnisetup: error renaming tmp binary file: %v", err)
		return err
	}

	return nil
}
