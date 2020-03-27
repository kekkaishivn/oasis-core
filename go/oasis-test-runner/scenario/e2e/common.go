// Package e2e implements the Oasis e2e test scenarios.
package e2e

import (
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/oasislabs/oasis-core/go/common"
	"github.com/oasislabs/oasis-core/go/common/logging"
	"github.com/oasislabs/oasis-core/go/common/node"
	"github.com/oasislabs/oasis-core/go/oasis-test-runner/env"
	"github.com/oasislabs/oasis-core/go/oasis-test-runner/oasis"
)

const (
	cfgNodeBinary       = "e2e.node.binary"
	cfgClientBinaryDir  = "e2e.client.binary_dir"
	cfgRuntimeBinaryDir = "e2e.runtime.binary_dir"
	cfgRuntimeLoader    = "e2e.runtime.loader"
	cfgTEEHardware      = "e2e.tee_hardware"
	cfgHaltEpoch        = "e2e.halt_epoch"
)

var (
	// Flags is the command line flags for the e2e tests.
	Flags = flag.NewFlagSet("", flag.ContinueOnError)

	// NoParameters is used when no parameters can be set for the scenario using CLI.
	NoParameters = make(map[string]interface{})

	runtimeID    common.Namespace
	keymanagerID common.Namespace

	logger = logging.GetLogger("e2e/common")
)

func resolveClientBinary(clientBinary string) string {
	return filepath.Join(viper.GetString(cfgClientBinaryDir), clientBinary)
}

func resolveRuntimeBinary(runtimeBinary string) (string, error) {
	var tee node.TEEHardware
	err := tee.FromString(viper.GetString(cfgTEEHardware))
	if err != nil {
		return "", err
	}

	var runtimeExt string
	switch tee {
	case node.TEEHardwareInvalid:
		runtimeExt = ""
	case node.TEEHardwareIntelSGX:
		runtimeExt = ".sgxs"
	}

	return filepath.Join(viper.GetString(cfgRuntimeBinaryDir), runtimeBinary+runtimeExt), nil
}

func resolveDefaultKeyManagerBinary() (string, error) {
	return resolveRuntimeBinary("oasis-core-keymanager-runtime")
}

func startClient(env *env.Env, net *oasis.Network, binary string, clientArgs []string) (*exec.Cmd, error) {
	clients := net.Clients()
	if len(clients) == 0 {
		return nil, fmt.Errorf("scenario/e2e: network has no client nodes")
	}

	d, err := env.NewSubDir("client")
	if err != nil {
		return nil, err
	}

	w, err := d.NewLogWriter("client.log")
	if err != nil {
		return nil, err
	}

	args := []string{
		"--node-address", "unix:" + clients[0].SocketPath(),
		"--runtime-id", runtimeID.String(),
	}
	args = append(args, clientArgs...)

	cmd := exec.Command(binary, args...)
	cmd.SysProcAttr = oasis.CmdAttrs
	cmd.Stdout = w
	cmd.Stderr = w

	logger.Info("launching client",
		"binary", binary,
		"args", strings.Join(args, " "),
	)

	if err = cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "scenario/e2e: failed to start client")
	}

	return cmd, nil
}

func init() {
	Flags.String(cfgNodeBinary, "oasis-node", "path to the node binary")
	Flags.String(cfgClientBinaryDir, "", "path to the client binaries directory")
	Flags.String(cfgRuntimeBinaryDir, "", "path to the runtime binaries directory")
	Flags.String(cfgRuntimeLoader, "oasis-core-runtime-loader", "path to the runtime loader")
	Flags.String(cfgTEEHardware, "", "TEE hardware to use")
	Flags.Uint64(cfgHaltEpoch, math.MaxUint64, "halt epoch height")
	_ = viper.BindPFlags(Flags)

	_ = runtimeID.UnmarshalHex("8000000000000000000000000000000000000000000000000000000000000000")
	_ = keymanagerID.UnmarshalHex("c000000000000000ffffffffffffffffffffffffffffffffffffffffffffffff")
}
