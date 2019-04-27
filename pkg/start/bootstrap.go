package start

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

type bootstrapControlPlane struct {
	assetDir	string
	podManifestPath	string
	ownedManifests	[]string
}

func newBootstrapControlPlane(assetDir, podManifestPath string) *bootstrapControlPlane {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	return &bootstrapControlPlane{assetDir: assetDir, podManifestPath: podManifestPath}
}
func (b *bootstrapControlPlane) Start() error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	UserOutput("Starting temporary bootstrap control plane...\n")
	if err := os.RemoveAll(bootstrapSecretsDir); err != nil {
		return err
	}
	secretsDir := filepath.Join(b.assetDir, assetPathSecrets)
	if _, err := copyDirectory(secretsDir, bootstrapSecretsDir, true); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(b.assetDir, assetPathAdminKubeConfig), filepath.Join(bootstrapSecretsDir, "kubeconfig"), true); err != nil {
		return err
	}
	manifestsDir := filepath.Join(b.assetDir, assetPathBootstrapManifests)
	ownedManifests, err := copyDirectory(manifestsDir, b.podManifestPath, false)
	b.ownedManifests = ownedManifests
	return err
}
func (b *bootstrapControlPlane) Teardown() error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	if b == nil {
		return nil
	}
	UserOutput("Tearing down temporary bootstrap control plane...\n")
	if err := os.RemoveAll(bootstrapSecretsDir); err != nil {
		return err
	}
	for _, manifest := range b.ownedManifests {
		if err := os.Remove(manifest); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	b.ownedManifests = nil
	return nil
}
func copyFile(src, dst string, overwrite bool) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	flags := os.O_CREATE | os.O_WRONLY
	if !overwrite {
		flags |= os.O_EXCL
	}
	dstfile, err := os.OpenFile(dst, flags, os.FileMode(0600))
	if err != nil {
		return err
	}
	defer dstfile.Close()
	srcfile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcfile.Close()
	_, err = io.Copy(dstfile, srcfile)
	return err
}
func copyDirectory(srcDir, dstDir string, overwrite bool) ([]string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	var copied []string
	return copied, filepath.Walk(srcDir, func(src string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		dst := filepath.Join(dstDir, strings.TrimPrefix(src, srcDir))
		if info.IsDir() {
			err = os.Mkdir(dst, os.FileMode(0700))
			if os.IsExist(err) {
				err = nil
			}
			return err
		}
		if err := copyFile(src, dst, overwrite); err != nil {
			return err
		}
		copied = append(copied, dst)
		return nil
	})
}
