// Doka Doka Builder Container!
package main

import (
	"context"
	"path"
	"runtime"
	"strings"
	
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	
	"github.com/aki-kuramoto/dokadoka-bc/internal/ddbccfg"
	"github.com/aki-kuramoto/dokadoka-bc/internal/ddbcmain"
	"github.com/docker/docker/client"
)

func main() {
	ddbcCfg, err := ddbccfg.ParseDdbcArgs(os.Args[1:])
	if err != nil {
		log.Fatalf("Failed to parse arguments: %v", err)
	}
	ctx := context.Background()
	
	gitRepo, localRepoPath, pathToDelete, err := ddbcmain.EnsureLocalRepo(
		ctx,
		ddbcCfg.LocalRepoRoot,
		ddbcCfg.SourceRepoUrl,
		ddbcCfg.SourceTag,
		ddbcCfg.GitSshUsername,
		ddbcCfg.GitSshPrivateKeyFilename,
		ddbcCfg.GitSshPrivateKeyPassphrase,
	)
	defer func(pathToDelete string) {
		// TODO: 一時ディレクトリを削除しないオプションを作る.
		err := os.RemoveAll(pathToDelete)
		if err != nil {
			log.Fatalf("Failed to remove the tmp dir: %v", err)
		}
	}(pathToDelete)
	_ = gitRepo
	if err != nil {
		log.Fatalf("Repo Error: %v", err)
	}
	
	var dockerClient *client.Client
	if strings.TrimSpace(ddbcCfg.DockerSshAddress) != "" {
		dockerClient, err = ddbcmain.NewSshTunneledDockerClient(
			ctx,
			ddbcCfg.DockerHost,
			ddbcCfg.DockerSshAddress,
			ddbcCfg.DockerSshUsername,
			ddbcCfg.DockerSshPrivateKeyFilename,
			ddbcCfg.DockerSshPrivateKeyPassphrase,
		)
	} else {
		dockerClient, err = client.NewClientWithOpts(
			client.WithHost(ddbcCfg.DockerHost),
			client.WithAPIVersionNegotiation(),
		)
	}
	if dockerClient == nil || err != nil {
		log.Fatalf("Docker client error: %v", err)
	}
	defer func(dockerClient *client.Client) {
		err := dockerClient.Close()
		if err != nil {
			log.Fatalf("Docker client close error: %v", err)
		}
	}(dockerClient)
	
	imageName, err := ddbcmain.EnsureContainerImage(
		ctx,
		dockerClient,
		ddbcCfg.ToolchainAndVer,
	)
	
	goModCachePath, err := ddbcmain.EnsureGoModCache(
		ctx,
		ddbcCfg,
	)
	
	for _, target := range ddbcCfg.Targets {
		err := processTarget(
			ctx,
			target,
			ddbcCfg,
			localRepoPath,
			dockerClient,
			imageName,
			goModCachePath,
		)
		if err != nil {
			fmt.Printf("Failed to process target %#v: %v", target, err)
		}
	}
	
	fmt.Println("DDBC Done.")
}

func processTarget(
	ctx context.Context,
	targetSpec *ddbccfg.TargetSpec,
	ddbcCfg *ddbccfg.DdbcConfig,
	localRepoPath string,
	dockerClient *client.Client,
	imageName string,
	goModCachePath string,
) error {
	binaryName := targetSpec.GetArtifactName()
	outputPath := filepath.Join(localRepoPath, binaryName)
	
	fmt.Printf("Building for %s/%s...\n", targetSpec.Os, targetSpec.Arch)
	
	if err := ddbcmain.BuildInContainer(
		ctx,
		dockerClient,
		localRepoPath,
		binaryName,
		targetSpec,
		ddbcCfg,
		imageName,
		goModCachePath,
	); err != nil {
		return fmt.Errorf("Build failed for %s: %v", binaryName, err)
	}
	
	if ddbcCfg.HasAnyStoreSpec() {
		fmt.Printf("Uploading %s to S3...\n", binaryName)
		
		objectKey := path.Join(ddbcCfg.StoreFolders, binaryName)
		if err := ddbcmain.UploadToS3(
			ctx,
			outputPath,
			objectKey,
			ddbcCfg,
		); err != nil {
			log.Printf("Upload failed for %s: %v", binaryName, err)
		}
	}
	
	localDestDir := strings.TrimSpace(ddbcCfg.LocalDestDir)
	if localDestDir != "" {
		if err := os.MkdirAll(localDestDir, 0755); err != nil {
			return fmt.Errorf("Failed to create local dest: %v", err)
		}
		
		destPath := filepath.Join(localDestDir, binaryName)
		if err := copyFile(outputPath, destPath); err != nil {
			log.Printf("Failed to copy binary to local: %v", err)
		} else {
			if runtime.GOOS == "darwin" {
				// only macOS due to the filesystem of Windows has no permissions
				os.Chmod(destPath, 0755)
			}
			fmt.Printf("Binary saved to: %s\n", destPath)
		}
	}
	
	return nil
}

func copyFile(
	srcPath string,
	dstPath string,
) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer func(srcFile *os.File) {
		err := srcFile.Close()
		if err != nil {
			fmt.Printf("srcFile close failed with %#v\n", err)
		}
	}(srcFile)
	
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer func(dstFile *os.File) {
		err := dstFile.Close()
		if err != nil {
			fmt.Printf("dstFile close failed with %#v\n", err)
		}
	}(dstFile)
	
	_, err = io.Copy(dstFile, srcFile)
	return err
}
