package httpapi

import (
	"time"

	"drift-detector-svc/internal/domain"
)

type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}

type ReadinessResponse struct {
	Status           string     `json:"status" example:"ready"`
	LastCycleID      string     `json:"lastCycleId,omitempty"`
	LastCycleAt      *time.Time `json:"lastCycleAt,omitempty"`
	LastCyclePartial bool       `json:"lastCyclePartial,omitempty"`
}

type APIError struct {
	Message string `json:"message" example:"detection cycle already running"`
}

type DetectionRunResponse struct {
	Status string            `json:"status" example:"completed"`
	Result DetectionCycleDTO `json:"result"`
}

type DetectionCycleDTO struct {
	CycleID                string                       `json:"cycleId"`
	Trigger                string                       `json:"trigger"`
	StartedAt              time.Time                    `json:"startedAt"`
	FinishedAt             time.Time                    `json:"finishedAt"`
	DurationMs             int64                        `json:"durationMs"`
	StageTimings           domain.DetectionStageTimings `json:"stageTimings"`
	Partial                bool                         `json:"partial"`
	InventoryMarkedPartial bool                         `json:"inventoryMarkedPartial"`
	ParserReady            bool                         `json:"parserReady"`
	Stats                  domain.DetectionStats        `json:"stats"`
	Warnings               []string                     `json:"warnings,omitempty"`
	ErrorMessages          []string                     `json:"errorMessages,omitempty"`
}

func toDetectionCycleDTO(result domain.DetectionCycleResult) DetectionCycleDTO {
	return DetectionCycleDTO{
		CycleID:                result.CycleID,
		Trigger:                result.Trigger,
		StartedAt:              result.StartedAt,
		FinishedAt:             result.FinishedAt,
		DurationMs:             result.DurationMs,
		StageTimings:           result.StageTimings,
		Partial:                result.Partial,
		InventoryMarkedPartial: result.InventoryMarkedPartial,
		ParserReady:            result.ParserReady,
		Stats:                  result.Stats,
		Warnings:               result.Warnings,
		ErrorMessages:          result.ErrorMessages,
	}
}
