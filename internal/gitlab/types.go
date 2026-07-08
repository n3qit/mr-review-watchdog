package gitlab

import "time"

// MergeRequest — подмножество полей GitLab merge request, необходимых для проверки.
type MergeRequest struct {
	IID            int       `json:"iid"`
	Title          string    `json:"title"`
	WebURL         string    `json:"web_url"`
	Author         User      `json:"author"`
	CreatedAt      time.Time `json:"created_at"`
	Draft          bool      `json:"draft"`
	WorkInProgress bool      `json:"work_in_progress"`
	State          string    `json:"state"`
}

// IsDraft сообщает, является ли МР черновиком (учитывает legacy-поле work_in_progress).
func (mr MergeRequest) IsDraft() bool {
	return mr.Draft || mr.WorkInProgress
}

type User struct {
	Username string `json:"username"`
}

type approvalsResponse struct {
	ApprovedBy []struct {
		User User `json:"user"`
	} `json:"approved_by"`
}

// Note — подмножество полей GitLab note (комментарий/системная заметка).
type Note struct {
	Author User `json:"author"`
	System bool `json:"system"`
}
