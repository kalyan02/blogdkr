package dropbox

type WebhookNotification struct {
	ListFolder *struct {
		Accounts []string `json:"accounts"`
	} `json:"list_folder,omitempty"`
	Delta *struct {
		Users []int `json:"users"`
	} `json:"delta,omitempty"`
}

type WebhookVerification struct {
	Challenge string `form:"challenge"`
}
