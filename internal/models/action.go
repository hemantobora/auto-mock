package models

type ActionType string

const (
	ActionView     ActionType = "view"
	ActionAdd      ActionType = "add"
	ActionEdit     ActionType = "edit"
	ActionRemove   ActionType = "remove"
	ActionDeploy   ActionType = "deploy"
	ActionDestroy  ActionType = "destroy"
	ActionCreate   ActionType = "create"
	ActionReplace  ActionType = "replace"
	ActionCancel   ActionType = "cancel"
	ActionGenerate ActionType = "generate"
	ActionDelete   ActionType = "delete"
	ActionDownload ActionType = "download"
	ActionExit     ActionType = "exit"
	ActionLocal    ActionType = "local"
	ActionSave     ActionType = "save"
)
