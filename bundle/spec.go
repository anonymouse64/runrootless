package bundle

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/opencontainers/runc/libcontainer/specconv"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

func transformSpec(spec *specs.Spec, oldBundle string) error {
	specconv.ToRootless(spec)
	toAbsoluteRootFS(spec, oldBundle)
	return injectPRoot(spec)
}

func toAbsoluteRootFS(spec *specs.Spec, oldBundle string) {
	if !filepath.IsAbs(spec.Root.Path) {
		spec.Root.Path = filepath.Clean(filepath.Join(oldBundle, spec.Root.Path))
	}
}

func injectPRoot(spec *specs.Spec) error {
	proot, err := prootPath()
	if err != nil {
		return err
	}

	// copy the file into /proot/proot instead of using a bind mount
	// using a bind mount doesn't work inside when running runc in a namespaced process group
	// such as when running from a snap
	err = exec.Command("cp", "--preserve=all", proot, filepath.Join(spec.Root.Path, "proot")).Run()
	if err != nil {
		return err
	}

	spec.Mounts = append(spec.Mounts,
		specs.Mount{
			Destination: "/dev/proot",
			Type:        "tmpfs",
			Source:      "tmpfs",
			Options:     []string{"exec", "mode=755", "size=32256k"},
		},
		// specs.Mount{
		// 	Destination: "/dev/proot/proot",
		// 	Type:        "bind",
		// 	Source:      proot,
		// 	Options:     []string{"bind", "ro"},
		// },
	)
	spec.Process.Args = append([]string{"/proot", "-0"}, spec.Process.Args...)
	spec.Process.Env = append(spec.Process.Env, "PROOT_TMP_DIR=/dev/proot")
	enableSeccomp, _ := strconv.ParseBool(os.Getenv("RUNROOTLESS_SECCOMP"))
	if !enableSeccomp {
		spec.Process.Env = append(spec.Process.Env, "PROOT_NO_SECCOMP=1")
	}
	return nil
}

func prootPath() (string, error) {
	// we can't use os/user.Current in a static binary.
	// moby/moby#29478
	home := os.Getenv("HOME")
	s := filepath.Join(home, ".runrootless", "runrootless-proot")
	_, err := os.Stat(s)
	if os.IsNotExist(err) {
		return s, errors.Errorf("%s not found. please install runrootless-proot according to README.", s)
	}
	return s, err
}
