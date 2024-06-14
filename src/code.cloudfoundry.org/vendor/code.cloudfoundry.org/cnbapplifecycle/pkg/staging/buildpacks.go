package staging

import (
	"context"
	"os"
	"runtime"
	"slices"
	"strings"

	"code.cloudfoundry.org/cnbapplifecycle/pkg/archive"
	"code.cloudfoundry.org/cnbapplifecycle/pkg/log"

	"github.com/BurntSushi/toml"
	"github.com/buildpacks/pack/pkg/blob"
	"github.com/buildpacks/pack/pkg/buildpack"
	"github.com/buildpacks/pack/pkg/dist"
)

type OrderTOML struct {
	Order dist.Order `toml:"order,omitempty"`
}

func DownloadBuildpacks(buildpacks []string, buildpacksDir string, imageFetcher buildpack.ImageFetcher, downloader blob.Downloader, orderFile *os.File, logger *log.Logger) error {
	fetchedBps := []buildpack.BuildModule{}
	order := dist.Order{{Group: []dist.ModuleRef{}}}
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
		order = appendToOrder(order, mainBp.Descriptor().Info())
	}

	if err := toml.NewEncoder(orderFile).Encode(OrderTOML{Order: order}); err != nil {
		return err
	}

	return extractBuildpacks(removeDuplicates(fetchedBps), buildpacksDir)
}

func appendToOrder(order dist.Order, bp dist.ModuleInfo) dist.Order {
	newOrder := dist.Order{}
	for _, e := range order {
		entry := e
		entry.Group = append(entry.Group, dist.ModuleRef{
			ModuleInfo: bp,
			Optional:   false,
		})
		newOrder = append(newOrder, entry)
	}

	return newOrder
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
		reader, err := bp.Open()
		if err != nil {
			return err
		}
		defer reader.Close()

		if err := archive.ExtractWithBaseOverride(reader, dist.BuildpacksDir, dir); err != nil {
			return err
		}
	}
	return nil
}
