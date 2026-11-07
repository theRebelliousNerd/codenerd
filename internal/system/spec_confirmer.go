package system

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/tools/codedom"
	"codenerd/internal/world"
)

// wireSpecLinkConfirmer registers the confirmer behind confirm_spec_link for
// this Cortex: the tool confirms a link in the workspace's store and in this
// kernel's spec rows, the ones the world scan loads at boot and the holographic
// spec block reads. Without a workspace or a kernel there are no spec links to
// confirm, and the tool says so.
func wireSpecLinkConfirmer(kernel core.Kernel, workspace string) {
	if workspace == "" || kernel == nil {
		codedom.RegisterSpecLinkConfirmer(nil)
		logging.Get(logging.CategoryBoot).Debug("spec link confirmer not registered: no workspace or kernel")
		return
	}
	codedom.RegisterSpecLinkConfirmer(world.NewSpecLinkConfirmer(kernel, workspace))
	logging.Get(logging.CategoryBoot).Debug("spec link confirmer registered for %s", workspace)
}
