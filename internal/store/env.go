package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"apiclient/internal/model"
)

// secretFile is the gitignored companion of an environment file.
type secretFile struct {
	Values map[string]string `yaml:"values"`
}

func (w *Workspace) envPaths(name string) (string, string, error) {
	if name == "" || strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return "", "", fmt.Errorf("invalid environment name %q", name)
	}
	dir := filepath.Join(w.Root, EnvDir)
	return filepath.Join(dir, name+".yaml"), filepath.Join(dir, name+secretSuffix), nil
}

func (w *Workspace) ListEnvironments() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(w.Root, EnvDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".yaml") || strings.HasSuffix(n, secretSuffix) {
			continue
		}
		out = append(out, strings.TrimSuffix(n, ".yaml"))
	}
	sort.Strings(out)
	return out, nil
}

// LoadEnvironment reads the committed file and fills secret values from the
// local secret file.
func (w *Workspace) LoadEnvironment(name string) (*model.Environment, error) {
	pub, sec, err := w.envPaths(name)
	if err != nil {
		return nil, err
	}
	env := &model.Environment{Name: name}
	if err := readYAML(pub, env); err != nil {
		return nil, err
	}
	var s secretFile
	if err := readYAML(sec, &s); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for i, v := range env.Variables {
		if v.Secret {
			env.Variables[i].Value = s.Values[v.Key]
		}
	}
	return env, nil
}

// SaveEnvironment writes non-secret values to <name>.yaml and secret values
// to <name>.secret.yaml (which Init adds to .gitignore).
func (w *Workspace) SaveEnvironment(env *model.Environment) error {
	pub, sec, err := w.envPaths(env.Name)
	if err != nil {
		return err
	}
	committed := model.Environment{Name: env.Name, Variables: make([]model.Variable, 0, len(env.Variables))}
	secrets := map[string]string{}
	for _, v := range env.Variables {
		if v.Secret {
			if v.Value != "" {
				secrets[v.Key] = v.Value
			}
			v.Value = ""
		}
		committed.Variables = append(committed.Variables, v)
	}
	if err := writeYAML(pub, &committed); err != nil {
		return err
	}
	if len(secrets) == 0 {
		if err := os.Remove(sec); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := writeYAML(sec, &secretFile{Values: secrets}); err != nil {
		return err
	}
	return os.Chmod(sec, 0o600)
}

func (w *Workspace) DeleteEnvironment(name string) error {
	pub, sec, err := w.envPaths(name)
	if err != nil {
		return err
	}
	for _, p := range []string{pub, sec} {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
