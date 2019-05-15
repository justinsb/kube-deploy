package imagebuilder

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"path/filepath"

	"k8s.io/kube-deploy/imagebuilder/pkg/imagebuilder/executor"
)

type Builder struct {
	config *Config
	target *executor.Target
}

func NewBuilder(config *Config, target *executor.Target) *Builder {
	return &Builder{
		config: config,
		target: target,
	}
}

func (b *Builder) RunSetupCommands() error {
	for _, c := range b.config.SetupCommands {
		if err := b.target.Exec(c...); err != nil {
			return err
		}
	}

	return nil
}

func (b *Builder) BuildImage(template []byte, extraEnv map[string]string, logdir string) error {
	tmpdir := fmt.Sprintf("/tmp/imagebuilder-%d", rand.Int63())
	err := b.target.Mkdir(tmpdir, 0755)
	if err != nil {
		return err
	}
	defer b.target.Exec("rm", "-rf", tmpdir)

	if logdir == "" {
		logdir = path.Join(tmpdir, "logs")
	}
	err = b.target.Mkdir(logdir, 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}

	if err := b.copyTree("forks/bootstrap-vz", tmpdir+"/bootstrap-vz"); err != nil {
		return err
	}

	//err = b.target.Exec("git", "clone", b.config.BootstrapVZRepo, "-b", b.config.BootstrapVZBranch, tmpdir+"/bootstrap-vz")
	//if err != nil {
	//	return err
	//}

	err = b.target.Put(tmpdir+"/template.yml", len(template), bytes.NewReader(template), 0644)
	if err != nil {
		return err
	}

	cmd := b.target.Command("./bootstrap-vz/bootstrap-vz", "--debug", "--log", logdir, "./template.yml")
	cmd.Cwd = tmpdir
	for k, v := range extraEnv {
		cmd.Env[k] = v
	}
	cmd.Sudo = true
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

// copyTree performs a fairly inefficient upload of our bootstrap-vz fork
// We don't expect a lot of files and don't expect any to be particularly big
func (b *Builder) copyTree(srcDir string, destDir string) error {
	files, err := ioutil.ListFiles(srcDir)
	if err != nil {
		return fmt.Errorrf("error listing files in %q: %v", srcDir, err)
	}

	for _, f := range files {
		srcChild := filepath.Join(srcDir, f.Name())
		destChild := path.Join(destDir, f.Name())

		if f.IsDir() {
			if err := b.target.Mkdir(destChild, f.Mode()); err != nil {
				return fmt.Errorf("error creating destination directory %q: %v", destChild, err)
			}

			if err := b.copyTree(srcChild, destChild); err != nil {
				return err
			}
		} else {
			// We don't expect any of these files to be big
			b, err := ioutil.ReadFile(srcChild)
			if err != nil {
				return fmt.Errorf("error reading %q: %v", srcChild, err)
			}

			if err := b.target.Put(destChild, len(b), bytes.NewReader(b), f.Mode()); err != nil {
				return fmt.Errorf("error writing %q: %v", destChild, err)
			}
		}
	}

	return nil
}
