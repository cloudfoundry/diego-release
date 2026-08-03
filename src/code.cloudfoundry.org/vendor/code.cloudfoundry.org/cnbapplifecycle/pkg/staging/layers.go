package staging

import (
	"code.cloudfoundry.org/cnbapplifecycle/pkg/log"
	"github.com/buildpacks/lifecycle/buildpack"
)

func RemoveBuildOnlyLayers(layersDir string, buildpacks []buildpack.GroupElement, logger *log.Logger) error {
	for _, bp := range buildpacks {
		bpDir, err := buildpack.ReadLayersDir(layersDir, bp, logger)
		logger.Debugf("processing buildpack directory %q", bpDir.Path)
		if err != nil {
			logger.Errorf("failed to read layers for buildpack %q at %q", bp.ID, bpDir.Path)
			return err
		}

		for _, layer := range bpDir.FindLayers(buildOnly) {
			logger.Debugf("removing layer %q", layer.Path())
			if err := layer.Remove(); err != nil {
				return err
			}
		}
	}

	return nil
}

func buildOnly(l buildpack.Layer) bool {
	md, err := l.Read()
	return err == nil && !md.Launch
}
