package main

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"

	"github.com/docker/cli/cli-plugins/manager"
	"github.com/docker/cli/cli-plugins/plugin"
	"github.com/docker/cli/cli/command"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
)

const (
	COMMAND_NAME = "call"

	LABEL_PREFIX = "com.docker.runtime"
)

var (
	dockerCliCommand = os.Getenv("DOCKER_CLI_PLUGIN_ORIGINAL_CLI_COMMAND")

	globalArgs = os.Args[1:int(math.Max(1, float64(slices.Index(os.Args, COMMAND_NAME))))]
)

func main() {
	plugin.Run(
		func(dockerCli command.Cli) *cobra.Command {
			var (
				workdir string
			)

			cmd := &cobra.Command{
				Use:   COMMAND_NAME,
				Short: "",
				Args:  cobra.MinimumNArgs(1),
				RunE: func(cmd *cobra.Command, args []string) error {
					image, restArgs := args[0], args[1:]

					imgId, err := getImage(workdir, image)
					if err != nil {
						return fmt.Errorf("failed to get image: %v", err)
					}

					img, _, err := dockerCli.Client().ImageInspectWithRaw(context.Background(), imgId)
					if client.IsErrNotFound(err) {
						// TODO use the API client
						runCmd := exec.Command(
							dockerCliCommand,
							slices.Concat(
								globalArgs,
								[]string{"pull"},
								[]string{imgId},
							)...,
						)
						runCmd.Stdout = os.Stderr
						runCmd.Stderr = os.Stderr

						if err := runCmd.Run(); err != nil {
							return fmt.Errorf("failed to pull image '%s': %v", imgId, err)
						}

						// Retry inspect
						img, _, err = dockerCli.Client().ImageInspectWithRaw(context.Background(), imgId)
					}
					if err != nil {
						return fmt.Errorf("failed to inspect image '%s': %v", imgId, err)
					}

					runFlags := toRunFlags(img, workdir)

					// TODO use the API client
					runCmd := exec.Command(
						dockerCliCommand,
						slices.Concat(
							globalArgs,
							[]string{"run", "--rm", "-it"},
							runFlags,
							[]string{img.ID},
							restArgs,
						)...,
					)
					runCmd.Dir = workdir
					runCmd.Stdout = os.Stdout
					runCmd.Stderr = os.Stderr
					runCmd.Stdin = os.Stdin

					return runCmd.Run()
				},
			}

			wd, err := os.Getwd()
			if err != nil {
				panic(fmt.Errorf("failed to get working directory: %w", err))
			}

			flags := cmd.Flags()
			flags.StringVarP(&workdir, "workdir", "w", wd, "Work dir")
			return cmd
		},
		manager.Metadata{
			SchemaVersion: "0.1.0",
			Vendor:        "Sergei Egorov",
			Version:       "0.0.1",
		},
	)
}

func getImage(workdir string, image string) (string, error) {
	parsedUrl, err := url.Parse(image)
	if err != nil {
		return "", fmt.Errorf("failed to parse image: %v", err)
	}

	switch parsedUrl.Scheme {
	case "":
		return image, nil
	case "file":
		dockerfile := path.Join(parsedUrl.Host, parsedUrl.Path)
		action := parsedUrl.Fragment

		tempImageIdFile, err := os.CreateTemp("", "image-id-*")
		if err != nil {
			return "", fmt.Errorf("failed to create temporary file: %v", err)
		}
		defer os.Remove(tempImageIdFile.Name())

		// TODO use the API client
		buildCmd := exec.Command(
			dockerCliCommand,
			append(globalArgs, "build", "-f", dockerfile, "--iidfile", tempImageIdFile.Name(), "--target", action, workdir)...,
		)
		buildCmd.Dir = workdir
		buildCmd.Stderr = os.Stderr
		buildCmd.Stdout = os.Stderr

		if err := buildCmd.Run(); err != nil {
			return "", fmt.Errorf("failed to get image id: %v", err)
		}

		tempImageIdFile, err = os.Open(tempImageIdFile.Name())
		if err != nil {
			return "", fmt.Errorf("failed to open temporary file: %v", err)
		}

		imgIdBytes, err := io.ReadAll(tempImageIdFile)
		if err != nil {
			return "", fmt.Errorf("failed to read image id: %v", err)
		}

		return string(imgIdBytes), nil
	default:
		return "", fmt.Errorf("unsupported scheme: %s", parsedUrl.Scheme)
	}
}

func toRunFlags(img types.ImageInspect, workdir string) []string {
	var runFlags []string

	for label, value := range img.Config.Labels {
		if _, ok := strings.CutPrefix(label, LABEL_PREFIX+".mounts."); ok {
			value = os.Expand(value, func(key string) string {
				switch key {
				case "workdir":
					return workdir
				default:
					return key
				}
			})
			runFlags = append(runFlags, "--mount", value)
			continue
		}

		if _, ok := strings.CutPrefix(label, LABEL_PREFIX+".ports."); ok {
			runFlags = append(runFlags, "-p", value)
			continue
		}

		if label == LABEL_PREFIX+".network" {
			runFlags = append(runFlags, "--network", value)
			continue
		}
	}

	for volume := range img.Config.Volumes {
		hash := sha1.New()
		hash.Write([]byte(workdir))
		hash.Write([]byte(volume))
		id := fmt.Sprintf("action-%x", hash.Sum(nil))

		runFlags = append(runFlags, "-v", fmt.Sprintf("%s:%s", id, volume))
	}

	runFlags = append(runFlags, "-e", fmt.Sprintf("workdir=%s", workdir))

	return runFlags
}
