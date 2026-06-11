// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mail

import (
	"context"
	"time"

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/log"
	"github.com/getpatchwork/patchwork/pkg/webhook"
)

const (
	eventCoverCreated        = "cover-created"
	eventPatchCreated        = "patch-created"
	eventPatchCompleted      = "patch-completed"
	eventSeriesCreated       = "series-created"
	eventSeriesCompleted     = "series-completed"
	eventCoverCommentCreated = "cover-comment-created"
	eventPatchCommentCreated = "patch-comment-created"
)

func (p *parser) queueEvent(e db.Event) {
	e.ProjectID = p.project.ID
	e.Date = time.Now()
	p.events = append(p.events, e)
	log.Debugf("event queued: %s", e.Category)
}

func (p *parser) createPatchCreatedEvent() {
	p.queueEvent(db.Event{
		Category: eventPatchCreated,
		PatchID:  db.Ptr[int32](p.patch.ID),
	})
}

func (p *parser) createCoverCreatedEvent(cover *db.Cover) {
	p.queueEvent(db.Event{
		Category: eventCoverCreated,
		CoverID:  db.Ptr[int32](cover.ID),
	})
}

func (p *parser) createSeriesCreatedEvent() {
	p.queueEvent(db.Event{
		Category: eventSeriesCreated,
		SeriesID: db.Ptr[int32](p.series.ID),
	})
}

func (p *parser) createPatchCommentCreatedEvent(comment *db.PatchComment, patch *db.Patch) {
	p.queueEvent(db.Event{
		Category:       eventPatchCommentCreated,
		PatchID:        db.Ptr[int32](patch.ID),
		PatchCommentID: db.Ptr[int32](comment.ID),
	})
}

func (p *parser) createCoverCommentCreatedEvent(comment *db.CoverComment, cover *db.Cover) {
	p.queueEvent(db.Event{
		Category:       eventCoverCommentCreated,
		CoverID:        db.Ptr[int32](cover.ID),
		CoverCommentID: db.Ptr[int32](comment.ID),
	})
}

func (p *parser) createPatchCompletedEvent() {
	if p.series == nil || p.patch == nil || p.patch.Number == nil {
		return
	}
	number := int(*p.patch.Number)
	if number <= 0 {
		return
	}

	predCount, err := p.db.CountPredecessorPatches(p.series.ID, number)
	if err != nil || predCount != number-1 {
		return
	}

	p.queueEvent(db.Event{
		Category: eventPatchCompleted,
		PatchID:  db.Ptr[int32](p.patch.ID),
		SeriesID: db.Ptr[int32](p.series.ID),
	})

	successors, err := p.db.GetSuccessorPatches(p.series.ID, number)
	if err != nil {
		return
	}
	count := number + 1
	for _, s := range successors {
		if s.Number == nil || int(*s.Number) != count {
			break
		}
		p.queueEvent(db.Event{
			Category: eventPatchCompleted,
			PatchID:  db.Ptr[int32](s.ID),
			SeriesID: db.Ptr[int32](p.series.ID),
		})
		count++
	}
}

func (p *parser) createSeriesCompletedEvent() {
	if p.series == nil || p.series.Total <= 0 {
		return
	}

	count, err := p.db.CountPatchesInSeries(p.series.ID)
	if err != nil {
		return
	}
	if count < int(p.series.Total) {
		return
	}

	p.queueEvent(db.Event{
		Category: eventSeriesCompleted,
		SeriesID: db.Ptr[int32](p.series.ID),
	})
}

func (p *parser) flushEvents(ctx context.Context, database *bun.DB) {
	if len(p.events) == 0 {
		return
	}

	q, err := db.Begin(ctx, database)
	if err != nil {
		log.Warnf("flush events: begin: %v", err)
		return
	}
	defer q.Rollback()

	for i := range p.events {
		if err := q.CreateEvent(&p.events[i]); err != nil {
			log.Warnf("event %s: %v", p.events[i].Category, err)
		} else {
			log.Debugf("event created: %s (id=%d)",
				p.events[i].Category, p.events[i].ID)
		}
	}

	hooks, _ := q.GetActiveWebhooks(p.project.ID)

	if err := q.Commit(); err != nil {
		log.Warnf("flush events: commit: %v", err)
		return
	}

	webhook.Deliver(ctx, database, p.events, hooks, p.project)
}
