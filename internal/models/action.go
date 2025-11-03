package models

type ActionType string

const (
	ActionView     ActionType = "view"
	ActionAdd      ActionType = "add"
	ActionEdit     ActionType = "edit"
	ActionRemove   ActionType = "remove"
	ActionDestroy  ActionType = "destroy"
	ActionCreate   ActionType = "create"
	ActionReplace  ActionType = "replace"
	ActionGenerate ActionType = "generate"
	ActionDelete   ActionType = "delete"
	ActionDownload ActionType = "download"
	ActionExit     ActionType = "exit"
	ActionLocal    ActionType = "local"
	ActionSave     ActionType = "save"
)
