// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"time"
)

const siteReplicationAPIVersion = "1"

type peerSite struct {
	Name      string `json:"name"`
	Endpoint  string `json:"endpoints"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
}

type syncStatus string

type bucketBandwidth struct {
	Limit     uint64    `json:"bandwidthLimitPerBucket"`
	IsSet     bool      `json:"set"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}

type peerInfo struct {
	Endpoint             string          `json:"endpoint"`
	Name                 string          `json:"name"`
	DeploymentID         string          `json:"deploymentID"`
	SyncState            syncStatus      `json:"sync"`
	DefaultBandwidth     bucketBandwidth `json:"defaultbandwidth"`
	ReplicateILMExpiry   bool            `json:"replicate-ilm-expiry"`
	ObjectNamingMode     string          `json:"objectNamingMode,omitempty"`
	TablesReplicaEnabled bool            `json:"tablesReplicaEnabled,omitempty"`
	APIVersion           string          `json:"apiVersion,omitempty"`
}

type replicateAddStatus struct {
	Success                 bool   `json:"success"`
	Status                  string `json:"status"`
	ErrDetail               string `json:"errorDetail,omitempty"`
	InitialSyncErrorMessage string `json:"initialSyncErrorMessage,omitempty"`
}

type replicateEditStatus struct {
	Success   bool   `json:"success"`
	Status    string `json:"status"`
	ErrDetail string `json:"errorDetail,omitempty"`
}

type replicateRemoveStatus struct {
	Status     string `json:"status"`
	ErrDetail  string `json:"errorDetail,omitempty"`
	APIVersion string `json:"apiVersion,omitempty"`
}

type siteReplicationInfo struct {
	Enabled                 bool       `json:"enabled"`
	Name                    string     `json:"name,omitempty"`
	Sites                   []peerInfo `json:"sites,omitempty"`
	ServiceAccountAccessKey string     `json:"serviceAccountAccessKey,omitempty"`
	APIVersion              string     `json:"apiVersion,omitempty"`
	RawJSON                 []byte     `json:"-"`
}

func (i *siteReplicationInfo) setRawJSON(data []byte) {
	i.RawJSON = append(i.RawJSON[:0], data...)
}

func (i siteReplicationInfo) rawJSON() []byte {
	return i.RawJSON
}

type srRemoveReq struct {
	RequestingDepID string   `json:"requestingDepID"`
	SiteNames       []string `json:"sites"`
	RemoveAll       bool     `json:"all"`
}

type srAddOptions struct {
	ReplicateILMExpiry bool
	Force              bool
}

type srEditOptions struct {
	DisableILMExpiryReplication bool
	EnableILMExpiryReplication  bool
}

type srStatusOptions struct {
	Buckets        bool
	Policies       bool
	Users          bool
	Groups         bool
	Metrics        bool
	ILMExpiryRules bool
	PeerState      bool
	Entity         srEntityType
	EntityValue    string
	ShowDeleted    bool
}

type srEntityType int

const (
	srEntityUnspecified srEntityType = iota
	srBucketEntity
	srPolicyEntity
	srUserEntity
	srGroupEntity
	srILMExpiryRuleEntity
)

func srEntityTypeForName(name string) srEntityType {
	switch name {
	case "bucket":
		return srBucketEntity
	case "policy":
		return srPolicyEntity
	case "user":
		return srUserEntity
	case "group":
		return srGroupEntity
	case "ilm-expiry-rule":
		return srILMExpiryRuleEntity
	default:
		return srEntityUnspecified
	}
}

type srStateInfo struct {
	Name       string              `json:"name"`
	Peers      map[string]peerInfo `json:"peers"`
	UpdatedAt  time.Time           `json:"updatedAt"`
	APIVersion string              `json:"apiVersion,omitempty"`
}

type srInfo struct {
	Enabled      bool        `json:"enabled"`
	Name         string      `json:"name"`
	DeploymentID string      `json:"deploymentID"`
	State        srStateInfo `json:"state"`
	APIVersion   string      `json:"apiVersion,omitempty"`
	RawJSON      []byte      `json:"-"`
}

func (i *srInfo) setRawJSON(data []byte) {
	i.RawJSON = append(i.RawJSON[:0], data...)
}

func (i srInfo) rawJSON() []byte {
	return i.RawJSON
}

type srStatusInfo struct {
	Enabled           bool                `json:"enabled"`
	MaxBuckets        int                 `json:"MaxBuckets"`
	MaxUsers          int                 `json:"MaxUsers"`
	MaxGroups         int                 `json:"MaxGroups"`
	MaxPolicies       int                 `json:"MaxPolicies"`
	MaxILMExpiryRules int                 `json:"MaxILMExpiryRules"`
	Sites             map[string]peerInfo `json:"Sites"`
	APIVersion        string              `json:"apiVersion,omitempty"`
	RawJSON           []byte              `json:"-"`
}

func (i *srStatusInfo) setRawJSON(data []byte) {
	i.RawJSON = append(i.RawJSON[:0], data...)
}

func (i srStatusInfo) rawJSON() []byte {
	return i.RawJSON
}

type rawJSONSetter interface {
	setRawJSON([]byte)
}

type rawJSONHolder interface {
	rawJSON() []byte
}

func rawJSONString(value any) (string, error) {
	if holder, ok := value.(rawJSONHolder); ok && len(holder.rawJSON()) > 0 {
		return string(holder.rawJSON()), nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
