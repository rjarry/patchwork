// Patchwork - automated patch tracking system
// Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

type User struct {
	bun.BaseModel `bun:"table:auth_user" json:"-"`

	ID          int32      `bun:"id,pk" json:"id"`
	Username    string     `bun:"username,notnull" json:"username"`
	Password    string     `bun:"password,notnull" json:"-"`
	FirstName   string     `bun:"first_name,notnull" json:"first_name"`
	LastName    string     `bun:"last_name,notnull" json:"last_name"`
	Email       string     `bun:"email,notnull" json:"email"`
	IsSuperuser bool       `bun:"is_superuser,notnull" json:"-"`
	IsStaff     bool       `bun:"is_staff,notnull" json:"-"`
	IsActive    bool       `bun:"is_active,notnull" json:"-"`
	DateJoined  time.Time  `bun:"date_joined,notnull" json:"-"`
	LastLogin   *time.Time `bun:"last_login" json:"-"`

	URL string `bun:"-" json:"url,omitempty"`
}

type Person struct {
	bun.BaseModel `bun:"table:patchwork_person" json:"-"`

	ID     int32   `bun:"id,pk,autoincrement" json:"id"`
	Email  string  `bun:"email,notnull,unique" json:"email"`
	Name   *string `bun:"name" json:"name"`
	UserID *int32  `bun:"user_id" json:"-"`

	User *User  `bun:"-" json:"user,omitempty"`
	URL  string `bun:"-" json:"url,omitempty"`
}

type Project struct {
	bun.BaseModel `bun:"table:patchwork_project" json:"-"`

	ID                   int32  `bun:"id,pk,autoincrement" json:"id"`
	Linkname             string `bun:"linkname,notnull,unique" json:"link_name"`
	Name                 string `bun:"name,notnull,unique" json:"name"`
	Listid               string `bun:"listid,notnull" json:"list_id"`
	Listemail            string `bun:"listemail,notnull" json:"list_email"`
	SubjectMatch         string `bun:"subject_match,notnull" json:"subject_match,omitempty"`
	WebURL               string `bun:"web_url,notnull" json:"web_url,omitempty"`
	ScmURL               string `bun:"scm_url,notnull" json:"scm_url,omitempty"`
	WebScmURL            string `bun:"webscm_url,notnull" json:"webscm_url,omitempty"`
	ListArchiveURL       string `bun:"list_archive_url,notnull" json:"list_archive_url,omitempty"`
	ListArchiveURLFormat string `bun:"list_archive_url_format,notnull" json:"list_archive_url_format,omitempty"`
	CommitURLFormat      string `bun:"commit_url_format,notnull" json:"commit_url_format,omitempty"`
	SendNotifications    bool   `bun:"send_notifications,notnull" json:"-"`
	UseTags              bool   `bun:"use_tags,notnull" json:"-"`
	ShowDependencies     bool   `bun:"show_dependencies,notnull" json:"show_dependencies,omitempty"`
	AutoSupersede        bool   `bun:"auto_supersede,notnull" json:"-"`

	APIURL      string `bun:"-" json:"url,omitempty"`
	Maintainers []User `bun:"-" json:"maintainers"`
}

type State struct {
	bun.BaseModel `bun:"table:patchwork_state" json:"-"`

	ID             int32  `bun:"id,pk,autoincrement" json:"id"`
	Name           string `bun:"name,notnull,unique" json:"name"`
	Slug           string `bun:"slug,notnull,unique" json:"slug"`
	Ordering       int32  `bun:"ordering,notnull,unique" json:"ordering"`
	ActionRequired bool   `bun:"action_required,notnull" json:"action_required"`
}

type Tag struct {
	bun.BaseModel `bun:"table:patchwork_tag" json:"-"`

	ID         int32  `bun:"id,pk,autoincrement" json:"id"`
	Name       string `bun:"name,notnull" json:"name"`
	Pattern    string `bun:"pattern,notnull" json:"-"`
	Abbrev     string `bun:"abbrev,notnull,unique" json:"abbrev"`
	ShowColumn bool   `bun:"show_column,notnull" json:"-"`
}

type PatchTag struct {
	bun.BaseModel `bun:"table:patchwork_patchtag" json:"-"`

	ID      int32 `bun:"id,pk,autoincrement" json:"id"`
	PatchID int32 `bun:"patch_id,notnull" json:"-"`
	TagID   int32 `bun:"tag_id,notnull" json:"-"`
	Count   int32 `bun:"count,notnull" json:"count"`
}

type DelegationRule struct {
	bun.BaseModel `bun:"table:patchwork_delegationrule" json:"-"`

	ID        int32  `bun:"id,pk,autoincrement" json:"id"`
	Path      string `bun:"path,notnull" json:"path"`
	Priority  int32  `bun:"priority,notnull" json:"priority"`
	ProjectID int32  `bun:"project_id,notnull" json:"-"`
	UserID    int32  `bun:"user_id,notnull" json:"-"`
}

type Cover struct {
	bun.BaseModel `bun:"table:patchwork_cover" json:"-"`

	ID          int32     `bun:"id,pk,autoincrement" json:"id"`
	Msgid       string    `bun:"msgid,notnull" json:"msgid"`
	Date        time.Time `bun:"date,notnull" json:"date"`
	Headers     string    `bun:"headers,notnull" json:"headers,omitempty"`
	SubmitterID int32     `bun:"submitter_id,notnull" json:"-"`
	Content     *string   `bun:"content" json:"content,omitempty"`
	ProjectID   int32     `bun:"project_id,notnull" json:"-"`
	Name        string    `bun:"name,notnull" json:"name"`

	Submitter *Person  `bun:"-" json:"submitter,omitempty"`
	Project   *Project `bun:"-" json:"project,omitempty"`

	URL            string      `bun:"-" json:"url,omitempty"`
	WebURL         string      `bun:"-" json:"web_url,omitempty"`
	MboxURL        string      `bun:"-" json:"mbox,omitempty"`
	ListArchiveURL string      `bun:"-" json:"list_archive_url,omitempty"`
	Comments       string      `bun:"-" json:"comments,omitempty"`
	SeriesList     []SeriesRef `bun:"-" json:"series"`
}

type Patch struct {
	bun.BaseModel `bun:"table:patchwork_patch" json:"-"`

	ID          int32     `bun:"id,pk,autoincrement" json:"id"`
	Msgid       string    `bun:"msgid,notnull" json:"msgid"`
	Date        time.Time `bun:"date,notnull" json:"date"`
	Headers     string    `bun:"headers,notnull" json:"headers,omitempty"`
	SubmitterID int32     `bun:"submitter_id,notnull" json:"-"`
	Content     *string   `bun:"content" json:"content,omitempty"`
	ProjectID   int32     `bun:"project_id,notnull" json:"-"`
	Name        string    `bun:"name,notnull" json:"name"`
	Diff        *string   `bun:"diff" json:"diff,omitempty"`
	CommitRef   *string   `bun:"commit_ref" json:"commit_ref,omitempty"`
	PullURL     *string   `bun:"pull_url" json:"pull_url,omitempty"`
	DelegateID  *int32    `bun:"delegate_id" json:"-"`
	StateID     *int32    `bun:"state_id" json:"-"`
	Archived    bool      `bun:"archived,notnull" json:"archived"`
	Hash        *string   `bun:"hash" json:"hash,omitempty"`
	SeriesID    *int32    `bun:"series_id" json:"-"`
	Number      *int16    `bun:"number" json:"-"`
	RelatedID   *int32    `bun:"related_id" json:"-"`

	Submitter *Person  `bun:"-" json:"submitter,omitempty"`
	Project   *Project `bun:"-" json:"project,omitempty"`
	State     *State   `bun:"-" json:"state,omitempty"`
	Delegate  *User    `bun:"-" json:"delegate,omitempty"`

	Related []PatchRef `bun:"-" json:"related"`

	URL            string         `bun:"-" json:"url,omitempty"`
	WebURL         string         `bun:"-" json:"web_url,omitempty"`
	MboxURL        string         `bun:"-" json:"mbox,omitempty"`
	ListArchiveURL string         `bun:"-" json:"list_archive_url,omitempty"`
	CommentsURL    string         `bun:"-" json:"comments,omitempty"`
	ChecksURL      string         `bun:"-" json:"checks,omitempty"`
	CombinedCheck  *string        `bun:"-" json:"check,omitempty"`
	CheckCounts    [4]int         `bun:"-" json:"-"`
	Tags           map[string]int `bun:"-" json:"tags"`
	SeriesList     []SeriesRef    `bun:"-" json:"series"`
}

type PatchRef struct {
	ID   int32  `json:"id"`
	URL  string `json:"url,omitempty"`
	Name string `json:"name"`
}

type SeriesRef struct {
	ID   int32   `json:"id"`
	URL  string  `json:"url,omitempty"`
	Name *string `json:"name"`
}

type Series struct {
	bun.BaseModel `bun:"table:patchwork_series" json:"-"`

	ID               int32     `bun:"id,pk,autoincrement" json:"id"`
	ProjectID        *int32    `bun:"project_id" json:"-"`
	CoverLetterID    *int32    `bun:"cover_letter_id" json:"-"`
	PreviousSeriesID *int32    `bun:"previous_series_id" json:"-"`
	Name             *string   `bun:"name" json:"name"`
	Date             time.Time `bun:"date,notnull" json:"date"`
	SubmitterID      int32     `bun:"submitter_id,notnull" json:"-"`
	Version          int32     `bun:"version,notnull" json:"version"`
	Total            int32     `bun:"total,notnull" json:"total"`

	Submitter *Person  `bun:"-" json:"submitter,omitempty"`
	Project   *Project `bun:"-" json:"project,omitempty"`

	URL            string            `bun:"-" json:"url,omitempty"`
	WebURL         string            `bun:"-" json:"web_url,omitempty"`
	MboxURL        string            `bun:"-" json:"mbox,omitempty"`
	ReceivedTotal  int               `bun:"-" json:"received_total"`
	ReceivedAll    bool              `bun:"-" json:"received_all"`
	CoverLetter    *Cover            `bun:"-" json:"cover_letter"`
	Patches        []Patch           `bun:"-" json:"patches"`
	Metadata       map[string]string `bun:"-" json:"metadata"`
	Dependencies   []string          `bun:"-" json:"dependencies"`
	Dependents     []string          `bun:"-" json:"dependents"`
	PreviousSeries *string           `bun:"-" json:"previous_series"`
	NextSeries     []string          `bun:"-" json:"next_series"`
}

type SeriesMetadata struct {
	bun.BaseModel `bun:"table:patchwork_seriesmetadata" json:"-"`

	ID       int32  `bun:"id,pk,autoincrement"`
	SeriesID int32  `bun:"series_id,notnull"`
	Key      string `bun:"key,notnull"`
	Value    string `bun:"value,notnull"`
}

type SeriesReference struct {
	bun.BaseModel `bun:"table:patchwork_seriesreference" json:"-"`

	ID        int32  `bun:"id,pk,autoincrement" json:"id"`
	Msgid     string `bun:"msgid,notnull" json:"msgid"`
	ProjectID int32  `bun:"project_id,notnull" json:"-"`
	SeriesID  int32  `bun:"series_id,notnull" json:"-"`
}

type PatchComment struct {
	bun.BaseModel `bun:"table:patchwork_patchcomment" json:"-"`

	ID          int32     `bun:"id,pk,autoincrement" json:"id"`
	Msgid       string    `bun:"msgid,notnull" json:"msgid"`
	Date        time.Time `bun:"date,notnull" json:"date"`
	Headers     string    `bun:"headers,notnull" json:"headers,omitempty"`
	SubmitterID int32     `bun:"submitter_id,notnull" json:"-"`
	Content     *string   `bun:"content" json:"content,omitempty"`
	PatchID     int32     `bun:"patch_id,notnull" json:"-"`
	Addressed   *bool     `bun:"addressed" json:"addressed"`

	Submitter *Person `bun:"-" json:"submitter,omitempty"`
	URL       string  `bun:"-" json:"url,omitempty"`
	Subject   string  `bun:"-" json:"subject,omitempty"`
}

type CoverComment struct {
	bun.BaseModel `bun:"table:patchwork_covercomment" json:"-"`

	ID          int32     `bun:"id,pk,autoincrement" json:"id"`
	Msgid       string    `bun:"msgid,notnull" json:"msgid"`
	Date        time.Time `bun:"date,notnull" json:"date"`
	Headers     string    `bun:"headers,notnull" json:"headers,omitempty"`
	SubmitterID int32     `bun:"submitter_id,notnull" json:"-"`
	Content     *string   `bun:"content" json:"content,omitempty"`
	CoverID     int32     `bun:"cover_id,notnull" json:"-"`
	Addressed   *bool     `bun:"addressed" json:"addressed"`

	Submitter *Person `bun:"-" json:"submitter,omitempty"`
	URL       string  `bun:"-" json:"url,omitempty"`
	Subject   string  `bun:"-" json:"subject,omitempty"`
}

type Event struct {
	bun.BaseModel `bun:"table:patchwork_event" json:"-"`

	ID                 int32     `bun:"id,pk,autoincrement" json:"id"`
	ProjectID          int32     `bun:"project_id,notnull" json:"-"`
	Category           string    `bun:"category,notnull" json:"category"`
	Date               time.Time `bun:"date,notnull" json:"date"`
	ActorID            *int32    `bun:"actor_id" json:"-"`
	PatchID            *int32    `bun:"patch_id" json:"-"`
	SeriesID           *int32    `bun:"series_id" json:"-"`
	CoverID            *int32    `bun:"cover_id" json:"-"`
	PreviousStateID    *int32    `bun:"previous_state_id" json:"-"`
	CurrentStateID     *int32    `bun:"current_state_id" json:"-"`
	PreviousDelegateID *int32    `bun:"previous_delegate_id" json:"-"`
	CurrentDelegateID  *int32    `bun:"current_delegate_id" json:"-"`
	PreviousRelationID *int32    `bun:"previous_relation_id" json:"-"`
	CurrentRelationID  *int32    `bun:"current_relation_id" json:"-"`
	CreatedCheckID     *int32    `bun:"created_check_id" json:"-"`
	CoverCommentID     *int32    `bun:"cover_comment_id" json:"-"`
	PatchCommentID     *int32    `bun:"patch_comment_id" json:"-"`

	Project *Project `bun:"-" json:"project,omitempty"`
	Actor   *User    `bun:"-" json:"actor"`
	Payload any      `bun:"-" json:"payload,omitempty"`
}

// CheckState maps Django's integer state to its string representation.
type CheckState int16

const (
	CheckPending CheckState = 0
	CheckSuccess CheckState = 1
	CheckWarning CheckState = 2
	CheckFail    CheckState = 3
)

func (s CheckState) MarshalJSON() ([]byte, error) {
	names := map[CheckState]string{
		CheckPending: "pending",
		CheckSuccess: "success",
		CheckWarning: "warning",
		CheckFail:    "fail",
	}
	name, ok := names[s]
	if !ok {
		name = "pending"
	}
	return json.Marshal(name)
}

func (s *CheckState) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}
	states := map[string]CheckState{
		"pending": CheckPending,
		"success": CheckSuccess,
		"warning": CheckWarning,
		"fail":    CheckFail,
	}
	*s = states[name]
	return nil
}

type Check struct {
	bun.BaseModel `bun:"table:patchwork_check" json:"-"`

	ID          int32      `bun:"id,pk,autoincrement" json:"id"`
	PatchID     int32      `bun:"patch_id,notnull" json:"-"`
	UserID      *int32     `bun:"user_id" json:"-"`
	Date        time.Time  `bun:"date,notnull" json:"date"`
	State       CheckState `bun:"state,notnull" json:"state"`
	TargetURL   string     `bun:"target_url,notnull" json:"target_url"`
	Context     string     `bun:"context,notnull" json:"context"`
	Description string     `bun:"description,notnull" json:"description"`

	User *User  `bun:"-" json:"user,omitempty"`
	URL  string `bun:"-" json:"url,omitempty"`
}

type Bundle struct {
	bun.BaseModel `bun:"table:patchwork_bundle" json:"-"`

	ID        int32  `bun:"id,pk,autoincrement" json:"id"`
	OwnerID   int32  `bun:"owner_id,notnull" json:"-"`
	ProjectID int32  `bun:"project_id,notnull" json:"-"`
	Name      string `bun:"name,notnull" json:"name"`
	Public    bool   `bun:"public,notnull" json:"public"`

	Owner   *User    `bun:"-" json:"owner,omitempty"`
	Project *Project `bun:"-" json:"project,omitempty"`

	URL           string  `bun:"-" json:"url,omitempty"`
	WebURL        string  `bun:"-" json:"web_url,omitempty"`
	MboxURL       string  `bun:"-" json:"mbox,omitempty"`
	BundlePatches []Patch `bun:"-" json:"patches"`
}

type PatchRelation struct {
	bun.BaseModel `bun:"table:patchwork_patchrelation" json:"-"`

	ID int32 `bun:"id,pk,autoincrement" json:"id"`
}

type BundlePatch struct {
	bun.BaseModel `bun:"table:patchwork_bundlepatch" json:"-"`

	ID       int32 `bun:"id,pk,autoincrement"`
	BundleID int32 `bun:"bundle_id,notnull"`
	PatchID  int32 `bun:"patch_id,notnull"`
	Order    int32 `bun:"order,notnull"`
}

type Webhook struct {
	bun.BaseModel `bun:"table:patchwork_webhook" json:"-"`

	ID        int32     `bun:"id,pk,autoincrement" json:"id"`
	ProjectID int32     `bun:"project_id,notnull" json:"-"`
	URL       string    `bun:"url,notnull" json:"url"`
	Secret    string    `bun:"secret,notnull" json:"-"`
	Events    string    `bun:"events,notnull" json:"events"`
	Active    bool      `bun:"active,notnull" json:"active"`
	CreatorID int32     `bun:"creator_id,notnull" json:"-"`
	Created   time.Time `bun:"created,notnull" json:"created"`
}

type UserProfile struct {
	bun.BaseModel `bun:"table:patchwork_userprofile" json:"-"`

	ID           int32 `bun:"id,pk,autoincrement" json:"id"`
	UserID       int32 `bun:"user_id,notnull,unique" json:"-"`
	SendEmail    bool  `bun:"send_email,notnull" json:"send_email"`
	ItemsPerPage int32 `bun:"items_per_page,notnull" json:"items_per_page"`
	ShowIds      bool  `bun:"show_ids,notnull" json:"show_ids"`
}
