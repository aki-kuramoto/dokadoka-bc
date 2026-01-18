package ddbccfg

import (
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/pflag"
)

type DdbcConfig struct {
	DockerHost                    string
	LocalDestDir                  string
	StoreEndpoint                 string
	StoreAccessKey                string
	StoreSecretKey                string
	StoreBucketName               string
	StoreFolders                  string
	StoreRegionName               string
	LocalRepoRoot                 string
	DockerSshAddress              string
	DockerSshUsername             string
	DockerSshPrivateKeyFilename   string
	DockerSshPrivateKeyPassphrase string
	SourceRepoUrl                 string
	SourceTag                     string
	GitSshUsername                string
	GitSshPrivateKeyFilename      string
	GitSshPrivateKeyPassphrase    string
	ToolchainAndVer               string
	// TODO: ビルドオプションにお節介をしないフラグを追加する.
	Targets []*TargetSpec
}

func newDdbcConfig(
	dockerHost string,
	localDestDir string,
	storeEndpoint string,
	storeAccessKey string,
	storeSecretKey string,
	storeBucketName string,
	storeFolders string,
	storeRegionName string,
	localRepoRoot string,
	dockerSshAddress string,
	dockerSshUsername string,
	dockerSshPrivateKeyFilename string,
	dockerSshPrivateKeyPassphrase string,
	sourceRepoUrl string,
	sourceTag string,
	gitSshUsername string,
	gitSshPrivateKeyFilename string,
	gitSshPrivateKeyPassphrase string,
	toolchainAndVer string,
	targets []*TargetSpec,
) *DdbcConfig {
	return &DdbcConfig{
		DockerHost:                    dockerHost,
		LocalDestDir:                  localDestDir,
		StoreEndpoint:                 storeEndpoint,
		StoreAccessKey:                storeAccessKey,
		StoreSecretKey:                storeSecretKey,
		StoreBucketName:               storeBucketName,
		StoreFolders:                  storeFolders,
		StoreRegionName:               storeRegionName,
		LocalRepoRoot:                 localRepoRoot,
		DockerSshAddress:              dockerSshAddress,
		DockerSshUsername:             dockerSshUsername,
		DockerSshPrivateKeyFilename:   dockerSshPrivateKeyFilename,
		DockerSshPrivateKeyPassphrase: dockerSshPrivateKeyPassphrase,
		SourceRepoUrl:                 sourceRepoUrl,
		SourceTag:                     sourceTag,
		GitSshUsername:                gitSshUsername,
		GitSshPrivateKeyFilename:      gitSshPrivateKeyFilename,
		GitSshPrivateKeyPassphrase:    gitSshPrivateKeyPassphrase,
		ToolchainAndVer:               toolchainAndVer,
		Targets:                       targets,
	}
}

// ParseDdbcArgs parses throughout opts and repeated (-t|--target) filename [TGT_OPTS]...
func ParseDdbcArgs(args []string) (*DdbcConfig, error) {
	err := godotenv.Load()
	if err != nil {
		log.Print("Failed to load `.env` file.")
		// .env is optional
	}

	// the weakest default Docker host depends on the host platform for no specs
	defaultDockerHost := defaultDockerHostByOs()
	// if the general DOCKER_HOST is presented, use it instead of the weakest default
	defaultDockerHost = envOrDefault("DOCKER_HOST", defaultDockerHost)
	// The following is treated later
	// if the DDBC specific DDBC_DOCKER_HOST, it wins over the lesser default
	// defaultDockerHost = envOrDefault("DDBC_DOCKER_HOST", defaultDockerHost)
	// finally `--docker-host` wins over everything else

	flagSet := pflag.NewFlagSet("ddbc", pflag.ContinueOnError)
	// prevent wasting target flags
	flagSet.SetInterspersed(false)

	var toOpts struct {
		dockerHost                    string
		localDestDir                  string
		storeEndpoint                 string
		storeAccessKey                string
		storeSecretKey                string
		storeBucketName               string
		storeFolders                  string
		storeRegionName               string
		localRepoRoot                 string
		dockerSshAddress              string
		dockerSshUsername             string
		dockerSshPrivateKeyFilename   string
		dockerSshPrivateKeyPassphrase string
		sourceRepoUrl                 string
		sourceTag                     string
		gitSshUsername                string
		gitSshPrivateKeyFilename      string
		gitSshPrivateKeyPassphrase    string
		toolchainAndVer               string
	}

	// throughout flags only
	flagSet.StringVar(&toOpts.dockerHost, "docker-host", envOrDefault("DDBC_DOCKER_HOST", defaultDockerHost), "docker host")
	flagSet.StringVar(&toOpts.localDestDir, "local-dest-dir", envOrDefault("DDBC_LOCAL_DEST_DIR", ""), "local artifact destination dir")
	flagSet.StringVar(&toOpts.storeEndpoint, "store-endpoint", envOrDefault("DDBC_STORE_ENDPOINT", ""), "store endpoint")
	flagSet.StringVar(&toOpts.storeAccessKey, "store-access-key", envOrDefault("DDBC_STORE_ACCESS_KEY", ""), "store access key")
	flagSet.StringVar(&toOpts.storeSecretKey, "store-secret-key", envOrDefault("DDBC_STORE_SECRET_KEY", ""), "store secret key")
	flagSet.StringVar(&toOpts.storeBucketName, "store-bucket-name", envOrDefault("DDBC_STORE_BUCKET_NAME", ""), "store bucket name")
	flagSet.StringVar(&toOpts.storeFolders, "store-folders", envOrDefault("DDBC_STORE_FOLDERS", ""), "store folders")
	flagSet.StringVar(&toOpts.storeRegionName, "store-region-name", envOrDefault("DDBC_STORE_REGION_NAME", ""), "store region name")
	flagSet.StringVar(&toOpts.localRepoRoot, "local-repo-root", envOrDefault("DDBC_LOCAL_REPO_ROOT", ""), "local repo root")
	flagSet.StringVar(&toOpts.dockerSshAddress, "docker-ssh-address", envOrDefault("DDBC_DOCKER_SSH_ADDRESS", ""), "docker ssh address")
	flagSet.StringVar(&toOpts.dockerSshUsername, "docker-ssh-username", envOrDefault("DDBC_DOCKER_SSH_USERNAME", ""), "docker ssh username")
	flagSet.StringVar(&toOpts.dockerSshPrivateKeyFilename, "docker-ssh-private-key-filename", envOrDefault("DDBC_DOCKER_SSH_PRIVATE_KEY_FILENAME", ""), "docker ssh private key filename")
	flagSet.StringVar(&toOpts.dockerSshPrivateKeyPassphrase, "docker-ssh-private-key-passphrase", envOrDefault("DDBC_DOCKER_SSH_PRIVATE_KEY_PASSPHRASE", ""), "docker ssh private key passphrase")
	flagSet.StringVar(&toOpts.sourceRepoUrl, "source-repo-url", envOrDefault("DDBC_SOURCE_REPO_URL", ""), "source repo url (required)")
	flagSet.StringVar(&toOpts.sourceTag, "source-tag", envOrDefault("DDBC_SOURCE_TAG", ""), "source tag/commit")
	flagSet.StringVar(&toOpts.gitSshUsername, "git-ssh-username", envOrDefault("DDBC_GIT_SSH_USERNAME", "git"), "git ssh username")
	flagSet.StringVar(&toOpts.gitSshPrivateKeyFilename, "git-ssh-private-key-filename", envOrDefault("DDBC_GIT_SSH_PRIVATE_KEY_FILENAME", ""), "git ssh private key filename")
	flagSet.StringVar(&toOpts.gitSshPrivateKeyPassphrase, "git-ssh-private-key-passphrase", envOrDefault("DDBC_GIT_SSH_PRIVATE_KEY_PASSPHRASE", ""), "git ssh private key passphrase")
	flagSet.StringVar(&toOpts.toolchainAndVer, "toolchain-and-ver", envOrDefault("DDBC_TOOLCHAIN_AND_VER", "golang:1.25.5"), "toolchain image and version")

	restOpts, err := parseKnownOnly(flagSet, args)
	if err != nil {
		return nil, err
	}

	// parse targets
	targets, err := parseTargets(restOpts)
	if err != nil {
		return nil, err
	}

	ddbcCfg := newDdbcConfig(
		toOpts.dockerHost,
		toOpts.localDestDir,
		toOpts.storeEndpoint,
		toOpts.storeAccessKey,
		toOpts.storeSecretKey,
		toOpts.storeBucketName,
		toOpts.storeFolders,
		toOpts.storeRegionName,
		toOpts.localRepoRoot,
		toOpts.dockerSshAddress,
		toOpts.dockerSshUsername,
		toOpts.dockerSshPrivateKeyFilename,
		toOpts.dockerSshPrivateKeyPassphrase,
		toOpts.sourceRepoUrl,
		toOpts.sourceTag,
		toOpts.gitSshUsername,
		toOpts.gitSshPrivateKeyFilename,
		toOpts.gitSshPrivateKeyPassphrase,
		toOpts.toolchainAndVer,
		targets,
	)

	if strings.TrimSpace(ddbcCfg.SourceRepoUrl) == "" {
		return nil, errors.New("DDBC_SOURCE_REPO_URL or --source-repo-url is required")
	}

	if ddbcCfg.LocalDestDir == "" && !ddbcCfg.HasAnyStoreSpec() {
		ddbcCfg.LocalDestDir = "."
	}

	if len(ddbcCfg.Targets) == 0 {
		return nil, errors.New("at least one target is required: `(-t|--target) filename [TGT_OPTS]...`")
	}

	return ddbcCfg, nil
}

// parseKnownOnly parses only flags defined in flagSet and returns the remaining args.
// This avoids pflag failing on target-specific flags before we reach them.
func parseKnownOnly(
	flagSet *pflag.FlagSet,
	args []string,
) ([]string, error) {
	// flagSet.Parse will stop at the first non-flag because SetInterspersed(false)
	// but our syntax can start with -t (flag). We want to stop when we hit -t/--target too.
	cut := len(args)

	for idx := 0; idx < len(args); idx += 1 {
		if args[idx] == "-t" || args[idx] == "--target" {
			cut = idx
			break
		}
	}

	if err := flagSet.Parse(args[:cut]); err != nil {
		return nil, err
	}

	return args[cut:], nil
}

func parseTargets(args []string) ([]*TargetSpec, error) {
	var targetSpecList []*TargetSpec

	idx := 0
	for idx < len(args) {
		if args[idx] != "-t" && args[idx] != "--target" {
			return nil, fmt.Errorf("unexpected token %q (expected -t/--target)", args[idx])
		}
		idx += 1

		if idx >= len(args) {
			return nil, errors.New("missing filename after -t/--target")
		}
		filename := args[idx]
		idx += 1

		// collect until next -t/--target or end
		startedAt := idx
		for (idx < len(args)) && (args[idx] != "-t") && (args[idx] != "--target") {
			idx += 1
		}
		tgtArgs := args[startedAt:idx]

		tgtFlagSet := pflag.NewFlagSet("target", pflag.ContinueOnError)
		tgtFlagSet.SetInterspersed(false)

		var tgtOs string
		var tgtArch string
		var tgtParams []string
		tgtFlagSet.StringVarP(&tgtOs, "os", "s", "", "target os")
		tgtFlagSet.StringVarP(&tgtArch, "arch", "a", "", "target architecture")
		tgtFlagSet.StringArrayVarP(&tgtParams, "param", "p", nil, "additional parameters to pass to the build command")

		if err := tgtFlagSet.Parse(tgtArgs); err != nil {
			return nil, err
		}

		if len(tgtFlagSet.Args()) != 0 {
			return nil, fmt.Errorf("unknown target args: %v", tgtFlagSet.Args())
		}

		targetSpecList = append(targetSpecList, &TargetSpec{
			Filename: filename,
			Os:       tgtOs,
			Arch:     tgtArch,
			Params:   tgtParams,
		})
	}

	return targetSpecList, nil
}

func envOrDefault(
	envName string,
	defaultValue string,
) string {
	if envValue, ok := os.LookupEnv(envName); ok {
		return envValue
	}

	return defaultValue
}

func defaultDockerHostByOs() string {
	if runtime.GOOS == "windows" {
		// named pipe for DockerDesktop for Windows and its compatibles
		return "npipe:////./pipe/docker_engine"
	}

	// for Linux, macOS, and other decent systems

	// check Rootless Docker sock existence
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		runtimeDockerSock := runtimeDir + "/docker.sock"
		if _, err := os.Stat(runtimeDockerSock); err == nil {
			rootlessPath := "unix://" + runtimeDockerSock
			return rootlessPath
		}
	}

	// the standard
	return "unix:///var/run/docker.sock"
}

func (ddbcCfg *DdbcConfig) HasAnyStoreSpec() bool {
	return isNotBlank(ddbcCfg.StoreEndpoint) ||
		isNotBlank(ddbcCfg.StoreAccessKey) ||
		isNotBlank(ddbcCfg.StoreSecretKey) ||
		isNotBlank(ddbcCfg.StoreBucketName) ||
		isNotBlank(ddbcCfg.StoreFolders) ||
		isNotBlank(ddbcCfg.StoreRegionName)
}

func isNotBlank(strToCheck string) bool {
	return strings.TrimSpace(strToCheck) != ""
}
