package ddbcmain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	
	cryptossh "golang.org/x/crypto/ssh"
	
	"github.com/aki-kuramoto/dokadoka-bc/internal/ddbccfg"
	"github.com/aki-kuramoto/dokadoka-bc/internal/encoderepopath"
)

// Note: when you meet the following problem,
// go: github.com/aki-kuramoto/dokadoka-bc/cmd/ddbc imports
//         github.com/docker/docker/api/types/container: github.com/docker/docker/api@v1.52.0: parsing go.mod:
//         module declares its path as: github.com/moby/moby/api
//                 but was required as: github.com/docker/docker/api
//
// -> Resolve the module path of Docker SDK explicitly
// $ go mod edit -replace github.com/docker/docker=github.com/docker/docker@v27.5.1+incompatible
// And be tidy again, Sayori!
// $ go mod tidy

func EnsureLocalRepo(
	ctx context.Context,
	localRepoRoot string,
	sourceRepoUrl string,
	tagOrHashOrBranch string,
	gitSshUsername string,
	gitSshPrivateKeyFilename string,
	gitSshPrivateKeyPassphrase string,

) (
	gitRepo *git.Repository,
	localRepoPath string,
	pathToDelete string,
	err error,
) {
	localRepoRoot = strings.TrimSpace(localRepoRoot)
	if localRepoRoot == "" {
		// use a temporary directory
		tmpDir, err := os.MkdirTemp("", "ddbc-*")
		if err != nil {
			log.Fatalf("Failed to create tmp dir: %v", err)
		}
		
		localRepoRoot = tmpDir
		pathToDelete = tmpDir
	}
	
	gitRepo, localRepoPath, err = cloneOrFetch(
		ctx,
		localRepoRoot,
		sourceRepoUrl,
		gitSshUsername,
		gitSshPrivateKeyFilename,
		gitSshPrivateKeyPassphrase,
	)
	if err != nil {
		return nil, "", pathToDelete, err
	}
	
	if err := checkoutTarget(
		ctx,
		gitRepo,
		tagOrHashOrBranch,
	); err != nil {
		log.Fatalf("Checkout Error: %v", err)
	}
	
	return gitRepo, localRepoPath, pathToDelete, nil
}

func EnsureContainerImage(
	ctx context.Context,
	dockerClient *client.Client,
	toolchainAndVer string,
) (string, error) {
	
	// imageName := "golang:1.22-alpine"
	imageName := "golang:1.25.5-alpine3.23"
	
	_, _, err := dockerClient.ImageInspectWithRaw(ctx, imageName)
	if err == nil {
		// OK, the image is available
		return imageName, nil
	}
	
	if err := pullImageAndWaitForCompletion(ctx, dockerClient, imageName); err != nil {
		// NG, failed to pull the image
		return "", err
	}
	
	// OK, it's ready
	return imageName, nil
}

func BuildInContainer(
	ctx context.Context,
	dockerClient *client.Client,
	localRepoPath string,
	binaryName string,
	targetSpec *ddbccfg.TargetSpec,
	ddbcCfg *ddbccfg.DdbcConfig,
	imageName string,
	goModCachePath string,
) error {
	
	// The container's side directory name
	// A long name like this is suitable for easy grasping. "src? which?"
	workingDirectoryName := "/shared-working-dir"
	
	dockerSshAddr := strings.TrimSpace(ddbcCfg.DockerSshAddress)
	
	goModCachePathOnContainer := goModCachePath
	if goModCachePathOnContainer != "" {
		if pathOnContainer, err := convertCachePathForContainerIfRequired(
			goModCachePath,
			dockerSshAddr,
		); err == nil {
			goModCachePathOnContainer = pathOnContainer
		}
	}
	
	command := []string{
		"go",
		"build",
		"-o",
		binaryName,
	}
	command = append(command, targetSpec.Params...)
	
	containerCfg := &container.Config{
		// User: will be set later (except Windows)
		Image: imageName,
		Env: []string{
			"GOOS=" + targetSpec.Os,
			"GOARCH=" + targetSpec.Arch,
			"CGO_ENABLED=0",
		},
		Cmd:        command,
		WorkingDir: workingDirectoryName,
	}
	const goPkgMod = "/go/pkg/mod"
	if goModCachePathOnContainer != "" {
		containerCfg.Env = append(containerCfg.Env, "GOMODCACHE="+goPkgMod)
	}
	
	if runtime.GOOS != "windows" {
		// On posix environment, specify the user and group.
		// (Avoid problem on Windows, Windows Host returns -1:-1)
		containerCfg.User = fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	}
	
	repoPathOnContainer, err := convertRepoPathForContainerIfRequired(
		localRepoPath,
		dockerSshAddr,
	)
	if err != nil {
		repoPathOnContainer = localRepoPath
	}
	
	hostCfg := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: repoPathOnContainer,
				Target: workingDirectoryName,
			},
		},
	}
	if goModCachePathOnContainer != "" {
		hostCfg.Mounts = append(hostCfg.Mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: goModCachePathOnContainer,
			Target: goPkgMod,
		})
	}
	
	containerCreateResponse, err := dockerClient.ContainerCreate(
		ctx,
		containerCfg,
		hostCfg,
		nil,
		nil,
		"",
	)
	if err != nil {
		// error during connect:
		// This may happen when the Docker service is down
		fmt.Printf("Client::ContainerCreate failed with %#v\n", err)
		return err
	}
	defer func(dockerClient *client.Client, ctx context.Context, containerID string, options container.RemoveOptions) {
		err := dockerClient.ContainerRemove(ctx, containerID, options)
		if err != nil {
			fmt.Printf("Client::ContainerRemove failed with %#v\n", err)
		}
	}(dockerClient, ctx, containerCreateResponse.ID, container.RemoveOptions{})
	
	if err := dockerClient.ContainerStart(
		ctx,
		containerCreateResponse.ID,
		container.StartOptions{},
	); err != nil {
		fmt.Printf("Client::ContainerStart failed with %#v\n", err)
		return err
	}
	
	logOptions := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		// like as the -f of `tail -f`
		Follow: true,
	}
	out, err := dockerClient.ContainerLogs(
		ctx,
		containerCreateResponse.ID,
		logOptions,
	)
	if err != nil {
		return err
	}
	
	go func() {
		defer func(out io.ReadCloser) {
			err := out.Close()
			if err != nil {
				fmt.Printf("close reader of Client::ContainerLogs failed with %#v\n", err)
			}
		}(out)
		
		_, _ = stdcopy.StdCopy(os.Stdout, os.Stderr, out)
	}()
	
	statusChan, errChan := dockerClient.ContainerWait(
		ctx,
		containerCreateResponse.ID,
		container.WaitConditionNotRunning,
	)
	select {
	case err := <-errChan:
		return err
	case status := <-statusChan:
		if status.StatusCode != 0 {
			return fmt.Errorf(
				"a builder container exited with non-zero status: %d. Check logs above",
				status.StatusCode,
			)
		}
	}
	
	return nil
}

func UploadToS3(
	ctx context.Context,
	targetFilePath string,
	targetSimpleName string,
	ddbcCfg *ddbccfg.DdbcConfig,
) error {
	
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if ddbcCfg.StoreEndpoint == "" {
			return aws.Endpoint{}, &aws.EndpointNotFoundError{}
		}
		
		return aws.Endpoint{
			URL:           ddbcCfg.StoreEndpoint,
			SigningRegion: ddbcCfg.StoreRegionName,
		}, nil
	})
	
	staticCredentialsProvider := credentials.NewStaticCredentialsProvider(
		ddbcCfg.StoreAccessKey,
		ddbcCfg.StoreSecretKey,
		"",
	)
	s3Cfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithCredentialsProvider(staticCredentialsProvider),
	)
	if err != nil {
		return err
	}
	
	s3Client := s3.NewFromConfig(
		s3Cfg,
		func(opts *s3.Options) {
			opts.UsePathStyle = true
		},
	)
	
	tgtFileReader, err := os.Open(targetFilePath)
	if err != nil {
		return err
	}
	defer func(tgtFileReader *os.File) {
		err := tgtFileReader.Close()
		if err != nil {
			fmt.Printf("close reader of target file failed with %#v\n", err)
		}
	}(tgtFileReader)
	
	objectKey := path.Join(ddbcCfg.StoreFolders, targetSimpleName)
	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(ddbcCfg.StoreBucketName),
		Key:    aws.String(objectKey),
		Body:   tgtFileReader,
	})
	
	if err != nil {
		return err
	}
	
	_, err = makeMetalink(ctx, targetSimpleName, ddbcCfg, s3Client)
	
	return err
}

func makeMetalink(
	ctx context.Context,
	targetSimpleName string,
	ddbcCfg *ddbccfg.DdbcConfig,
	s3Client *s3.Client,
) (string, error) {
	objectKey := path.Join(ddbcCfg.StoreFolders, targetSimpleName)
	if strings.TrimSpace(ddbcCfg.StoreMetalinkPrefix) == "" && strings.TrimSpace(ddbcCfg.StoreMetalinkSuffix) == "" {
		return "", nil
	}
	
	metalinkObjectKey := ddbcCfg.StoreMetalinkPrefix + targetSimpleName + ddbcCfg.StoreMetalinkSuffix
	metalinkDir := path.Dir(metalinkObjectKey)
	rel, err := relPath(metalinkDir, objectKey)
	if err != nil {
		return "", err
	}
	
	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(ddbcCfg.StoreBucketName),
		Key:          aws.String(metalinkObjectKey),
		Body:         nil,
		CacheControl: aws.String("no-cache, no-store, must-revalidate"),
		Metadata:     map[string]string{"link-to": rel},
	})
	if err != nil {
		return "", err
	}
	
	return rel, nil
}

// NewSshTunneledDockerClient - Creates a new SSH-tunneled Docker client
func NewSshTunneledDockerClient(
	ctx context.Context,
	dockerHost string,
	sshAddress string,
	username string,
	privateKeyFilename string,
	privateKeyPassphrase string,
) (*client.Client, error) {
	privateKeyFilenameToPass := expandHomeIfRequired(privateKeyFilename)
	privateKeyContents, err := os.ReadFile(privateKeyFilenameToPass)
	if err != nil {
		return nil, err
	}
	
	signer, err := cryptossh.ParsePrivateKey(privateKeyContents)
	if err != nil {
		passphrase := []byte(privateKeyPassphrase)
		signer, err = cryptossh.ParsePrivateKeyWithPassphrase(privateKeyContents, passphrase)
		if err != nil {
			return nil, err
		}
	}
	
	sshConfig := &cryptossh.ClientConfig{
		User: username,
		Auth: []cryptossh.AuthMethod{
			cryptossh.PublicKeys(signer),
		},
		HostKeyCallback: cryptossh.InsecureIgnoreHostKey(),
	}
	
	sshClient, err := cryptossh.Dial("tcp", sshAddress, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("ssh connection failed: %w", err)
	}
	
	remoteSocketPath := dockerHost
	if asUrl, err := url.Parse(dockerHost); err != nil {
		remoteSocketPath = strings.TrimPrefix(dockerHost, "unix://")
	} else {
		if asUrl.Scheme == "unix" {
			remoteSocketPath = asUrl.Path
		}
	}
	
	dialer := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return sshClient.Dial("unix", remoteSocketPath)
	}
	
	result, err := client.NewClientWithOpts(
		client.WithHost(dockerHost),
		client.WithDialContext(dialer),
		client.WithAPIVersionNegotiation(),
	)
	return result, err
}

func EnsureGoModCache(
	ctx context.Context,
	ddbcCfg *ddbccfg.DdbcConfig,
) (string, error) {
	
	goModCachePath := strings.TrimSpace(ddbcCfg.Gomodcache)
	if goModCachePath == "" {
		return "", nil
	}
	
	absGoModCachePath, err := filepath.Abs(goModCachePath)
	if err != nil {
		fmt.Printf("Warning: Failed to get absolute path for %s: %v\n", goModCachePath, err)
	}
	
	_, err = os.Stat(absGoModCachePath)
	os.IsNotExist(err)
	if err == nil {
		return absGoModCachePath, nil
	}
	
	if !os.IsNotExist(err) {
		return "", err
	}
	
	if err = os.MkdirAll(absGoModCachePath, 0755); err != nil && !os.IsExist(err) {
		log.Printf("Failed to ensure go mod cache dir: %v", err)
		return "", err
	}
	
	return absGoModCachePath, nil
}

func getGitAuth(
	gitSshPrivateKeyFilename string,
	gitSshUsername string,
	gitSshPrivateKeyPassword string,
) transport.AuthMethod {
	if gitSshPrivateKeyFilename == "" {
		// no auth
		return nil
	}
	
	privateKeyFilenameToPass := expandHomeIfRequired(gitSshPrivateKeyFilename)
	
	publicKeys, err := gitssh.NewPublicKeysFromFile(gitSshUsername, privateKeyFilenameToPass, gitSshPrivateKeyPassword)
	if err != nil {
		log.Printf("Warning: Failed to load SSH key: %v", err)
		return nil
	}
	
	// Avoid errors around known-hosts
	publicKeys.HostKeyCallback = cryptossh.InsecureIgnoreHostKey()
	return publicKeys
}

func cloneOrFetch(
	ctx context.Context,
	localRepoRoot string,
	sourceRepoUrl string,
	gitSshUsername string,
	gitSshPrivateKeyFilename string,
	gitSshPrivateKeyPassphrase string,
) (*git.Repository, string, error) {
	gitAuth := getGitAuth(
		gitSshPrivateKeyFilename,
		gitSshUsername,
		gitSshPrivateKeyPassphrase,
	)
	
	encodedRepoDir := encoderepopath.Encode(sourceRepoUrl)
	localRepoPath := filepath.Join(localRepoRoot, encodedRepoDir)
	var err error
	localRepoPath, err = filepath.Abs(localRepoPath)
	if err != nil {
		fmt.Printf("Warning: Failed to get absolute path for %s: %v\n", localRepoPath, err)
	}
	dotGitDir := filepath.Join(localRepoPath, ".git")
	
	if _, err := os.Stat(dotGitDir); os.IsNotExist(err) {
		if err = os.MkdirAll(localRepoRoot, 0755); err != nil && !os.IsExist(err) {
			log.Fatalf("Failed to ensure root dir: %v", err)
		}
		
		fmt.Printf("Cloning new repository into %s...\n", localRepoPath)
		
		opts := &git.CloneOptions{
			URL:      sourceRepoUrl,
			Auth:     gitAuth,
			Progress: os.Stdout,
		}
		
		gitRepo, err := git.PlainClone(localRepoPath, false, opts)
		if err != nil {
			return nil, "", err
		}
		
		return gitRepo, "", nil
	}
	
	fmt.Printf("Opening existing repository in %s...\n", localRepoPath)
	
	gitRepo, err := git.PlainOpen(localRepoPath)
	if err != nil {
		return nil, "", err
	}
	
	fmt.Println("Fetching updates...")
	err = gitRepo.Fetch(&git.FetchOptions{
		Auth:     gitAuth,
		Progress: os.Stdout,
		Force:    true,
	})
	if err != nil {
		if !errors.Is(err, git.NoErrAlreadyUpToDate) {
			// actual error
			return nil, "", err
		}
	}
	
	return gitRepo, localRepoPath, nil
}

func checkoutTarget(
	ctx context.Context,
	gitRepo *git.Repository,
	tagOrHash string,
) error {
	gitWt, err := gitRepo.Worktree()
	if err != nil {
		return err
	}
	
	const (
		TryAsHash = iota
		TryAsTag
		TryAsBranch
	)
	
	for i := TryAsHash; i <= TryAsBranch; i++ {
		checkoutOpts := &git.CheckoutOptions{}
		
		switch i {
		case TryAsHash:
			checkoutOpts.Hash = plumbing.NewHash(tagOrHash)
			checkoutOpts.Force = true
		case TryAsTag:
			checkoutOpts.Branch = plumbing.ReferenceName("refs/tags/" + tagOrHash)
			checkoutOpts.Force = true
		case TryAsBranch:
			checkoutOpts.Branch = plumbing.ReferenceName("refs/heads/" + tagOrHash)
			checkoutOpts.Force = true
		}
		err = gitWt.Checkout(checkoutOpts)
		if err == nil {
			break
		}
	}
	
	if err != nil {
		return fmt.Errorf("failed to checkout %s: %v", tagOrHash, err)
	}
	
	ref, _ := gitRepo.Head()
	fmt.Printf("Checked out at: %s (%s)\n", tagOrHash, ref.Hash().String()[:8])
	return nil
}

func pullImageAndWaitForCompletion(
	ctx context.Context,
	dockerClient *client.Client,
	imageName string,
) error {
	reader, err := dockerClient.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return err
	}
	defer func(reader io.ReadCloser) {
		err := reader.Close()
		if err != nil {
			_, err := fmt.Fprintf(os.Stderr, "Failed to close the reader of pulling the image %s: %v", imageName, err)
			if err != nil {
				return
			}
		}
	}(reader)
	
	var wg sync.WaitGroup
	wg.Add(1)
	
	go func() {
		defer wg.Done()
		_, _ = io.Copy(os.Stdout, reader)
	}()
	
	fmt.Printf("Pulling %s in background...\n", imageName)
	
	wg.Wait()
	fmt.Println("Pull completed.")
	
	return nil
}

func convertRepoPathForContainerIfRequired(
	hostRepoPath string,
	dockerSshAddr string,
) (string, error) {
	return convertPathForContainerIfRequired(hostRepoPath, dockerSshAddr)
}

func convertCachePathForContainerIfRequired(
	hostCachePath string,
	dockerSshAddr string,
) (string, error) {
	return convertPathForContainerIfRequired(hostCachePath, dockerSshAddr)
}

func convertPathForContainerIfRequired(
	hostPath string,
	dockerSshAddr string,
) (string, error) {
	if runtime.GOOS != "windows" {
		// as-is
		return hostPath, nil
	}
	
	if dockerSshAddr == "" {
		// For DockerDesktop for Windows and its compatibles,
		// They will convert it well probably
		return hostPath, nil
	}
	
	isLocalSsh := strings.Contains(dockerSshAddr, "localhost") ||
		strings.Contains(dockerSshAddr, "127.0.0.1") ||
		strings.Contains(dockerSshAddr, "[::1]")
	if !isLocalSsh {
		fmt.Println("warn: DDBC currently lacks additional mount features, the process will fail due to an isolated filesystem later")
		// for ignoring the error, both return
		return hostPath, errors.New("remote Docker connection detected: DDBC currently lacks additional mount features, the process will fail due to an isolated filesystem later")
	}
	
	pathSlashSeparated := filepath.ToSlash(hostPath)
	if len(pathSlashSeparated) >= 2 && pathSlashSeparated[1] == ':' {
		// Windows native form to WSL mounted form
		driveLetter := strings.ToLower(string(pathSlashSeparated[0]))
		mangledPath := "/mnt/" + driveLetter + pathSlashSeparated[2:]
		return mangledPath, nil
	}
	
	return pathSlashSeparated, nil
}

func expandHomeIfRequired(path string) string {
	switch runtime.GOOS {
	case "windows", "darwin":
		break
	case "linux", "freebsd", "netbsd", "openbsd", "solaris", "dragonfly":
		return path
	default:
		// If you want to add an OS to this manual expansion, please provide the details.
		return path
	}
	
	if !strings.HasPrefix(path, "~") {
		return path
	}
	
	currUser, err := user.Current()
	if err != nil {
		fmt.Println("Error getting current user:", err)
		return path
	}
	
	result := filepath.Join(currUser.HomeDir, strings.TrimPrefix(path, "~"))
	return result
}

func relPath(
	basepath string,
	tgtPath string,
) (string, error) {
	const separator = byte('/')
	
	baseClean := path.Clean(basepath)
	tgtClean := path.Clean(tgtPath)
	if strings.EqualFold(tgtClean, baseClean) {
		return ".", nil
	}
	
	if baseClean == "." {
		baseClean = ""
	}
	
	baseSlashed := len(baseClean) > 0 && baseClean[0] == separator
	tgtSlashed := len(tgtClean) > 0 && tgtClean[0] == separator
	if baseSlashed != tgtSlashed {
		return "", errors.New("relPath: can't make " + tgtPath + " relative to " + basepath)
	}
	
	bl := len(baseClean)
	tl := len(tgtClean)
	var b0, bi, t0, ti int
	for {
		for bi < bl && baseClean[bi] != separator {
			bi++
		}
		for ti < tl && tgtClean[ti] != separator {
			ti++
		}
		if !strings.EqualFold(tgtClean[t0:ti], baseClean[b0:bi]) {
			break
		}
		if bi < bl {
			bi++
		}
		if ti < tl {
			ti++
		}
		b0 = bi
		t0 = ti
	}
	if baseClean[b0:bi] == ".." {
		return "", errors.New("relPath: can't make " + tgtPath + " relative to " + basepath)
	}
	if b0 != bl {
		// Base elements left. Must go up before going down.
		seps := countNeedle(baseClean[b0:bl], separator)
		size := 2 + seps*3
		if tl != t0 {
			size += 1 + tl - t0
		}
		buf := make([]byte, size)
		n := copy(buf, "..")
		for i := 0; i < seps; i++ {
			buf[n] = separator
			copy(buf[n+1:], "..")
			n += 3
		}
		if t0 != tl {
			buf[n] = separator
			copy(buf[n+1:], tgtClean[t0:])
		}
		return string(buf), nil
	}
	return tgtClean[t0:], nil
}

func countNeedle(
	haystack string,
	needleByte byte,
) int {
	asByteSlice := []byte(haystack)
	counter := 0
	for _, b := range asByteSlice {
		if b == needleByte {
			counter += 1
		}
	}
	return counter
}
