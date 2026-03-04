package campaign

// isMutatingTaskType identifies task types that may mutate campaign-controlled
// workspace state and should therefore participate in write-set locking and
// scoped rollback.
func isMutatingTaskType(taskType TaskType) bool {
	switch taskType {
	case TaskTypeFileCreate,
		TaskTypeFileModify,
		TaskTypeTestWrite,
		TaskTypeDocument,
		TaskTypeRefactor,
		TaskTypeIntegrate,
		TaskTypeToolCreate,
		TaskTypeAssaultDiscover,
		TaskTypeAssaultTriage:
		return true
	default:
		return false
	}
}
