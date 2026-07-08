package buildpacks

import (
	"context"
	"os"
	"runtime"
	"slices"
	"strings"

	"code.cloudfoundry.org/cnbapplifecycle/pkg/archive"
	"code.cloudfoundry.org/cnbapplifecycle/pkg/log"

	"github.com/BurntSushi/toml"
	lifecycle "github.com/buildpacks/lifecycle/buildpack"
	"github.com/buildpacks/pack/pkg/blob"
	"github.com/buildpacks/pack/pkg/buildpack"
	"github.com/buildpacks/pack/pkg/dist"
)

type OrderTOML struct {
	Order lifecycle.Order `toml:"order,omitempty"`
}

func DownloadBuildpacks(buildpacks []string, buildpacksDir string, imageFetcher buildpack.ImageFetcher, downloader blob.Downloader, orderFile *os.File, autoDetect bool, logger *log.Logger) error {
	fetchedBps := []buildpack.BuildModule{}
	order := lifecycle.Order{}
	downloadOptions := buildpack.DownloadOptions{
		Daemon: false,
		Target: &dist.Target{
			OS:   "linux",
			Arch: runtime.GOARCH,
		},
	}

	bpDownloader := buildpack.NewDownloader(logger, imageFetcher, downloader, nil)

	logger.Infof("Using buildpacks: %s", strings.Join(buildpacks, ", "))
	for _, bp := range buildpacks {
		mainBp, depBps, err := bpDownloader.Download(context.Background(), bp, downloadOptions)
		if err != nil {
			return err
		}

		fetchedBps = append(append(fetchedBps, mainBp), depBps...)
		order = appendToOrder(order, mainBp.Descriptor().Info(), autoDetect)
	}

	if err := toml.NewEncoder(orderFile).Encode(OrderTOML{Order: order}); err != nil {
		return err
	}

	return extractBuildpacks(removeDuplicates(fetchedBps), buildpacksDir)
}

func appendToOrder(order lifecycle.Order, bp dist.ModuleInfo, autoDetect bool) lifecycle.Order {
	groupElement := lifecycle.GroupElement{
		ID:       bp.ID,
		Version:  bp.Version,
		Homepage: bp.Homepage,
		Optional: false,
	}

	if len(order) == 0 {
		return lifecycle.Order{
			lifecycle.Group{
				Group: []lifecycle.GroupElement{groupElement},
			},
		}
	}

	if autoDetect {
		return append(order, lifecycle.Group{
			Group: []lifecycle.GroupElement{groupElement},
		})
	}

	newGroup := order[0].Append(lifecycle.Group{
		Group: []lifecycle.GroupElement{groupElement},
	})

	return lifecycle.Order{newGroup}
}

func removeDuplicates(buildpacks []buildpack.BuildModule) []buildpack.BuildModule {
	result := []buildpack.BuildModule{}

	for _, bp := range buildpacks {
		if !slices.ContainsFunc(result, func(b buildpack.BuildModule) bool {
			return b.Descriptor().Info().FullName() == bp.Descriptor().Info().FullName()
		}) {
			result = append(result, bp)
		}
	}

	return result
}

func extractBuildpacks(buildpacks []buildpack.BuildModule, dir string) error {
	for _, bp := range buildpacks {
		err := extractBuildpack(bp, dir)
		if err != nil {
			return err
		}
	}
	return nil
}

func extractBuildpack(bp buildpack.BuildModule, dir string) error {
	reader, err := bp.Open()
	if err != nil {
		return err
	}
	defer reader.Close()

	return archive.ExtractWithBaseOverride(reader, dist.BuildpacksDir, dir)
}
