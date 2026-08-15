package logging

// =============================================================================
// CONVENIENCE FUNCTIONS - Quick logging without getting a logger first
// These are no-ops if the category is disabled
// =============================================================================

// Boot logs to the boot category
func Boot(format string, args ...any) {
	Get(CategoryBoot).Info(format, args...)
}

// BootDebug logs debug to the boot category
func BootDebug(format string, args ...any) {
	Get(CategoryBoot).Debug(format, args...)
}

// Session logs to the session category
func Session(format string, args ...any) {
	Get(CategorySession).Info(format, args...)
}

// SessionDebug logs debug to the session category
func SessionDebug(format string, args ...any) {
	Get(CategorySession).Debug(format, args...)
}

// Kernel logs to the kernel category
func Kernel(format string, args ...any) {
	Get(CategoryKernel).Info(format, args...)
}

// KernelDebug logs debug to the kernel category
func KernelDebug(format string, args ...any) {
	Get(CategoryKernel).Debug(format, args...)
}

// API logs to the api category
func API(format string, args ...any) {
	Get(CategoryAPI).Info(format, args...)
}

// APIDebug logs debug to the api category
func APIDebug(format string, args ...any) {
	Get(CategoryAPI).Debug(format, args...)
}

// Perception logs to the perception category
func Perception(format string, args ...any) {
	Get(CategoryPerception).Info(format, args...)
}

// PerceptionDebug logs debug to the perception category
func PerceptionDebug(format string, args ...any) {
	Get(CategoryPerception).Debug(format, args...)
}

// Articulation logs to the articulation category
func Articulation(format string, args ...any) {
	Get(CategoryArticulation).Info(format, args...)
}

// ArticulationDebug logs debug to the articulation category
func ArticulationDebug(format string, args ...any) {
	Get(CategoryArticulation).Debug(format, args...)
}

// Routing logs to the routing category
func Routing(format string, args ...any) {
	Get(CategoryRouting).Info(format, args...)
}

// RoutingDebug logs debug to the routing category
func RoutingDebug(format string, args ...any) {
	Get(CategoryRouting).Debug(format, args...)
}

// Tools logs to the tools category
func Tools(format string, args ...any) {
	Get(CategoryTools).Info(format, args...)
}

// ToolsDebug logs debug to the tools category
func ToolsDebug(format string, args ...any) {
	Get(CategoryTools).Debug(format, args...)
}

// VirtualStore logs to the virtual_store category
func VirtualStore(format string, args ...any) {
	Get(CategoryVirtualStore).Info(format, args...)
}

// VirtualStoreDebug logs debug to the virtual_store category
func VirtualStoreDebug(format string, args ...any) {
	Get(CategoryVirtualStore).Debug(format, args...)
}

// Shards logs to the shards category
func Shards(format string, args ...any) {
	Get(CategoryShards).Info(format, args...)
}

// ShardsDebug logs debug to the shards category
func ShardsDebug(format string, args ...any) {
	Get(CategoryShards).Debug(format, args...)
}

// Coder logs to the coder category
func Coder(format string, args ...any) {
	Get(CategoryCoder).Info(format, args...)
}

// CoderDebug logs debug to the coder category
func CoderDebug(format string, args ...any) {
	Get(CategoryCoder).Debug(format, args...)
}

// Tester logs to the tester category
func Tester(format string, args ...any) {
	Get(CategoryTester).Info(format, args...)
}

// TesterDebug logs debug to the tester category
func TesterDebug(format string, args ...any) {
	Get(CategoryTester).Debug(format, args...)
}

// Reviewer logs to the reviewer category
func Reviewer(format string, args ...any) {
	Get(CategoryReviewer).Info(format, args...)
}

// ReviewerDebug logs debug to the reviewer category
func ReviewerDebug(format string, args ...any) {
	Get(CategoryReviewer).Debug(format, args...)
}

// Researcher logs to the researcher category
func Researcher(format string, args ...any) {
	Get(CategoryResearcher).Info(format, args...)
}

// ResearcherDebug logs debug to the researcher category
func ResearcherDebug(format string, args ...any) {
	Get(CategoryResearcher).Debug(format, args...)
}

// SystemShards logs to the system_shards category
func SystemShards(format string, args ...any) {
	Get(CategorySystemShards).Info(format, args...)
}

// SystemShardsDebug logs debug to the system_shards category
func SystemShardsDebug(format string, args ...any) {
	Get(CategorySystemShards).Debug(format, args...)
}

// Dream logs to the dream category
func Dream(format string, args ...any) {
	Get(CategoryDream).Info(format, args...)
}

// DreamDebug logs debug to the dream category
func DreamDebug(format string, args ...any) {
	Get(CategoryDream).Debug(format, args...)
}

// Autopoiesis logs to the autopoiesis category
func Autopoiesis(format string, args ...any) {
	Get(CategoryAutopoiesis).Info(format, args...)
}

// AutopoiesisDebug logs debug to the autopoiesis category
func AutopoiesisDebug(format string, args ...any) {
	Get(CategoryAutopoiesis).Debug(format, args...)
}

// Campaign logs to the campaign category
func Campaign(format string, args ...any) {
	Get(CategoryCampaign).Info(format, args...)
}

// CampaignDebug logs debug to the campaign category
func CampaignDebug(format string, args ...any) {
	Get(CategoryCampaign).Debug(format, args...)
}

// Context logs to the context category
func Context(format string, args ...any) {
	Get(CategoryContext).Info(format, args...)
}

// ContextDebug logs debug to the context category
func ContextDebug(format string, args ...any) {
	Get(CategoryContext).Debug(format, args...)
}

// World logs to the world category
func World(format string, args ...any) {
	Get(CategoryWorld).Info(format, args...)
}

// WorldDebug logs debug to the world category
func WorldDebug(format string, args ...any) {
	Get(CategoryWorld).Debug(format, args...)
}

// Embedding logs to the embedding category
func Embedding(format string, args ...any) {
	Get(CategoryEmbedding).Info(format, args...)
}

// EmbeddingDebug logs debug to the embedding category
func EmbeddingDebug(format string, args ...any) {
	Get(CategoryEmbedding).Debug(format, args...)
}

// Store logs to the store category
func Store(format string, args ...any) {
	Get(CategoryStore).Info(format, args...)
}

// StoreDebug logs debug to the store category
func StoreDebug(format string, args ...any) {
	Get(CategoryStore).Debug(format, args...)
}

// Browser logs to the browser category
func Browser(format string, args ...any) {
	Get(CategoryBrowser).Info(format, args...)
}

// BrowserDebug logs debug to the browser category
func BrowserDebug(format string, args ...any) {
	Get(CategoryBrowser).Debug(format, args...)
}

// BrowserWarn logs warning to the browser category
func BrowserWarn(format string, args ...any) {
	Get(CategoryBrowser).Warn(format, args...)
}

// BrowserError logs error to the browser category
func BrowserError(format string, args ...any) {
	Get(CategoryBrowser).Error(format, args...)
}

// Tactile logs to the tactile category
func Tactile(format string, args ...any) {
	Get(CategoryTactile).Info(format, args...)
}

// TactileDebug logs debug to the tactile category
func TactileDebug(format string, args ...any) {
	Get(CategoryTactile).Debug(format, args...)
}

// TactileWarn logs warning to the tactile category
func TactileWarn(format string, args ...any) {
	Get(CategoryTactile).Warn(format, args...)
}

// TactileError logs error to the tactile category
func TactileError(format string, args ...any) {
	Get(CategoryTactile).Error(format, args...)
}

// JIT logs to the jit category
func JIT(format string, args ...any) {
	Get(CategoryJIT).Info(format, args...)
}

// JITDebug logs debug to the jit category
func JITDebug(format string, args ...any) {
	Get(CategoryJIT).Debug(format, args...)
}

// JITWarn logs warning to the jit category
func JITWarn(format string, args ...any) {
	Get(CategoryJIT).Warn(format, args...)
}

// JITError logs error to the jit category
func JITError(format string, args ...any) {
	Get(CategoryJIT).Error(format, args...)
}

// Build logs to the build category
func Build(format string, args ...any) {
	Get(CategoryBuild).Info(format, args...)
}

// BuildDebug logs debug to the build category
func BuildDebug(format string, args ...any) {
	Get(CategoryBuild).Debug(format, args...)
}

// BuildWarn logs warning to the build category
func BuildWarn(format string, args ...any) {
	Get(CategoryBuild).Warn(format, args...)
}

// BuildError logs error to the build category
func BuildError(format string, args ...any) {
	Get(CategoryBuild).Error(format, args...)
}

// =============================================================================
// WARN/ERROR CONVENIENCE FUNCTIONS - For remaining categories
// =============================================================================

// BootWarn logs warning to the boot category
func BootWarn(format string, args ...any) {
	Get(CategoryBoot).Warn(format, args...)
}

// BootError logs error to the boot category
func BootError(format string, args ...any) {
	Get(CategoryBoot).Error(format, args...)
}

// SessionWarn logs warning to the session category
func SessionWarn(format string, args ...any) {
	Get(CategorySession).Warn(format, args...)
}

// SessionError logs error to the session category
func SessionError(format string, args ...any) {
	Get(CategorySession).Error(format, args...)
}

// KernelWarn logs warning to the kernel category
func KernelWarn(format string, args ...any) {
	Get(CategoryKernel).Warn(format, args...)
}

// KernelError logs error to the kernel category
func KernelError(format string, args ...any) {
	Get(CategoryKernel).Error(format, args...)
}

// APIWarn logs warning to the api category
func APIWarn(format string, args ...any) {
	Get(CategoryAPI).Warn(format, args...)
}

// APIError logs error to the api category
func APIError(format string, args ...any) {
	Get(CategoryAPI).Error(format, args...)
}

// PerceptionWarn logs warning to the perception category
func PerceptionWarn(format string, args ...any) {
	Get(CategoryPerception).Warn(format, args...)
}

// PerceptionError logs error to the perception category
func PerceptionError(format string, args ...any) {
	Get(CategoryPerception).Error(format, args...)
}

// ArticulationWarn logs warning to the articulation category
func ArticulationWarn(format string, args ...any) {
	Get(CategoryArticulation).Warn(format, args...)
}

// ArticulationError logs error to the articulation category
func ArticulationError(format string, args ...any) {
	Get(CategoryArticulation).Error(format, args...)
}

// RoutingWarn logs warning to the routing category
func RoutingWarn(format string, args ...any) {
	Get(CategoryRouting).Warn(format, args...)
}

// RoutingError logs error to the routing category
func RoutingError(format string, args ...any) {
	Get(CategoryRouting).Error(format, args...)
}

// ToolsWarn logs warning to the tools category
func ToolsWarn(format string, args ...any) {
	Get(CategoryTools).Warn(format, args...)
}

// ToolsError logs error to the tools category
func ToolsError(format string, args ...any) {
	Get(CategoryTools).Error(format, args...)
}

// VirtualStoreWarn logs warning to the virtual_store category
func VirtualStoreWarn(format string, args ...any) {
	Get(CategoryVirtualStore).Warn(format, args...)
}

// VirtualStoreError logs error to the virtual_store category
func VirtualStoreError(format string, args ...any) {
	Get(CategoryVirtualStore).Error(format, args...)
}

// ShardsWarn logs warning to the shards category
func ShardsWarn(format string, args ...any) {
	Get(CategoryShards).Warn(format, args...)
}

// ShardsError logs error to the shards category
func ShardsError(format string, args ...any) {
	Get(CategoryShards).Error(format, args...)
}

// CoderWarn logs warning to the coder category
func CoderWarn(format string, args ...any) {
	Get(CategoryCoder).Warn(format, args...)
}

// CoderError logs error to the coder category
func CoderError(format string, args ...any) {
	Get(CategoryCoder).Error(format, args...)
}

// TesterWarn logs warning to the tester category
func TesterWarn(format string, args ...any) {
	Get(CategoryTester).Warn(format, args...)
}

// TesterError logs error to the tester category
func TesterError(format string, args ...any) {
	Get(CategoryTester).Error(format, args...)
}

// ReviewerWarn logs warning to the reviewer category
func ReviewerWarn(format string, args ...any) {
	Get(CategoryReviewer).Warn(format, args...)
}

// ReviewerError logs error to the reviewer category
func ReviewerError(format string, args ...any) {
	Get(CategoryReviewer).Error(format, args...)
}

// ResearcherWarn logs warning to the researcher category
func ResearcherWarn(format string, args ...any) {
	Get(CategoryResearcher).Warn(format, args...)
}

// ResearcherError logs error to the researcher category
func ResearcherError(format string, args ...any) {
	Get(CategoryResearcher).Error(format, args...)
}

// SystemShardsWarn logs warning to the system_shards category
func SystemShardsWarn(format string, args ...any) {
	Get(CategorySystemShards).Warn(format, args...)
}

// SystemShardsError logs error to the system_shards category
func SystemShardsError(format string, args ...any) {
	Get(CategorySystemShards).Error(format, args...)
}

// DreamWarn logs warning to the dream category
func DreamWarn(format string, args ...any) {
	Get(CategoryDream).Warn(format, args...)
}

// DreamError logs error to the dream category
func DreamError(format string, args ...any) {
	Get(CategoryDream).Error(format, args...)
}

// AutopoiesisWarn logs warning to the autopoiesis category
func AutopoiesisWarn(format string, args ...any) {
	Get(CategoryAutopoiesis).Warn(format, args...)
}

// AutopoiesisError logs error to the autopoiesis category
func AutopoiesisError(format string, args ...any) {
	Get(CategoryAutopoiesis).Error(format, args...)
}

// CampaignWarn logs warning to the campaign category
func CampaignWarn(format string, args ...any) {
	Get(CategoryCampaign).Warn(format, args...)
}

// CampaignError logs error to the campaign category
func CampaignError(format string, args ...any) {
	Get(CategoryCampaign).Error(format, args...)
}

// ContextWarn logs warning to the context category
func ContextWarn(format string, args ...any) {
	Get(CategoryContext).Warn(format, args...)
}

// ContextError logs error to the context category
func ContextError(format string, args ...any) {
	Get(CategoryContext).Error(format, args...)
}

// WorldWarn logs warning to the world category
func WorldWarn(format string, args ...any) {
	Get(CategoryWorld).Warn(format, args...)
}

// WorldError logs error to the world category
func WorldError(format string, args ...any) {
	Get(CategoryWorld).Error(format, args...)
}

// EmbeddingWarn logs warning to the embedding category
func EmbeddingWarn(format string, args ...any) {
	Get(CategoryEmbedding).Warn(format, args...)
}

// EmbeddingError logs error to the embedding category
func EmbeddingError(format string, args ...any) {
	Get(CategoryEmbedding).Error(format, args...)
}

// StoreWarn logs warning to the store category
func StoreWarn(format string, args ...any) {
	Get(CategoryStore).Warn(format, args...)
}

// StoreError logs error to the store category
func StoreError(format string, args ...any) {
	Get(CategoryStore).Error(format, args...)
}

// Persist logs to the persist category (fact snapshot write/read).
func Persist(format string, args ...any) {
	Get(CategoryPersist).Info(format, args...)
}

// PersistDebug logs debug to the persist category.
func PersistDebug(format string, args ...any) {
	Get(CategoryPersist).Debug(format, args...)
}

// PersistWarn logs warning to the persist category.
func PersistWarn(format string, args ...any) {
	Get(CategoryPersist).Warn(format, args...)
}

// Northstar logs to the northstar category (vision guardian).
// CategoryNorthstar was declared but had no wrapper, so every call site had to
// go through Get() while every neighbouring category had a one-liner — which is
// why the category ended up with almost no callers.
func Northstar(format string, args ...any) {
	Get(CategoryNorthstar).Info(format, args...)
}

// NorthstarDebug logs debug to the northstar category.
func NorthstarDebug(format string, args ...any) {
	Get(CategoryNorthstar).Debug(format, args...)
}

// NorthstarWarn logs warning to the northstar category.
func NorthstarWarn(format string, args ...any) {
	Get(CategoryNorthstar).Warn(format, args...)
}

// NorthstarError logs error to the northstar category.
func NorthstarError(format string, args ...any) {
	Get(CategoryNorthstar).Error(format, args...)
}

// Regression logs to the regression category (regression battery runs).
func Regression(format string, args ...any) {
	Get(CategoryRegression).Info(format, args...)
}

// RegressionDebug logs debug to the regression category.
func RegressionDebug(format string, args ...any) {
	Get(CategoryRegression).Debug(format, args...)
}

// RegressionWarn logs warning to the regression category.
func RegressionWarn(format string, args ...any) {
	Get(CategoryRegression).Warn(format, args...)
}

// PersistError logs error to the persist category.
func PersistError(format string, args ...any) {
	Get(CategoryPersist).Error(format, args...)
}
