package main

// seed creates an idempotent demo dataset: demo@okane.dev with a checking
// account and a wallet plus ~180 days of COP transactions (weekly groceries,
// monthly subscriptions and salary). Running it twice is a no-op.

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kenma/okane/internal/auth"
	"github.com/kenma/okane/internal/config"
	"github.com/kenma/okane/internal/domain"
	"github.com/kenma/okane/internal/finance"
	"github.com/kenma/okane/internal/money"
)

const (
	demoEmail    = "demo@okane.dev"
	demoPassword = "demo1234"
	demoName     = "Demo"
)

func seed(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool) error {
	loc, err := cfg.Location()
	if err != nil {
		return err
	}

	authSvc := auth.NewService(pool, cfg.SessionTTL)
	user, _, regErr := authSvc.Register(ctx, demoEmail, demoPassword, demoName)
	switch {
	case regErr == nil:
	case errors.Is(regErr, auth.ErrEmailTaken):
		user, err = authSvc.Store().GetUserByEmail(ctx, demoEmail)
	default:
		err = regErr
	}
	if err != nil {
		return err
	}

	svc := finance.NewService(finance.NewStore(pool), loc)
	accounts, err := svc.ListAccounts(ctx, user.ID)
	if err != nil {
		return err
	}
	if len(accounts) > 0 {
		fmt.Printf("demo data already present for %s; nothing to do\n", user.Email)
		return nil
	}

	categories, err := svc.ListCategories(ctx, user.ID)
	if err != nil {
		return err
	}
	bySlug := make(map[string]domain.Category, len(categories))
	for _, c := range categories {
		bySlug[c.Slug] = c
	}
	category := bySlug["groceries"].ID
	subsCat := bySlug["subscriptions"].ID
	salaryCat := bySlug["salary"].ID
	if category == uuid.Nil || subsCat == uuid.Nil || salaryCat == uuid.Nil {
		return errors.New("expected system categories (groceries, subscriptions, salary) are missing; run migrate first")
	}

	opening, err := money.Parse("1200000")
	if err != nil {
		return err
	}
	walletOpening, err := money.Parse("80000")
	if err != nil {
		return err
	}
	bancolombia, err := svc.CreateAccount(ctx, user.ID, finance.NewAccount{
		Name:            "Bancolombia",
		Type:            "checking",
		Currency:        "COP",
		OpeningBalance:  opening,
		InstitutionName: "Bancolombia",
	})
	if err != nil {
		return err
	}
	nequi, err := svc.CreateAccount(ctx, user.ID, finance.NewAccount{
		Name:            "Nequi",
		Type:            "other",
		Currency:        "COP",
		OpeningBalance:  walletOpening,
		InstitutionName: "Nequi",
	})
	if err != nil {
		return err
	}

	created := 0
	add := func(in finance.NewTransaction) error {
		if _, err := svc.CreateTransaction(ctx, user.ID, in); err != nil {
			return err
		}
		created++
		return nil
	}

	out := func(day time.Time, categoryID uuid.UUID, description, merchant string, accountID uuid.UUID, amount string) error {
		value, err := money.Parse(amount)
		if err != nil {
			return err
		}
		return add(finance.NewTransaction{
			AccountID:   accountID,
			CategoryID:  &categoryID,
			Description: description,
			Merchant:    &merchant,
			Amount:      value,
			Direction:   domain.DirectionOut,
			Currency:    "COP",
			Status:      domain.StatusPosted,
			OccurredAt:  &day,
		})
	}

	// Deterministic pseudo-randomness keeps repeated seed runs comparable.
	rnd := rand.New(rand.NewSource(42))
	places := []struct {
		name           string
		minCOP, maxCOP int
	}{
		{"D1", 32000, 130000},
		{"Ara", 25000, 85000},
		{"Éxito", 140000, 320000},
		{"Carulla", 160000, 360000},
	}

	type sub struct {
		name   string
		day    int
		amount string
		acc    uuid.UUID
	}
	subs := []sub{
		{name: "Netflix", day: 5, amount: "42000", acc: bancolombia.ID},
		{name: "Movistar", day: 15, amount: "78000", acc: bancolombia.ID},
		{name: "Spotify", day: 9, amount: "10500", acc: nequi.ID},
	}

	start := time.Now().In(loc).AddDate(0, 0, -180)
	now := time.Now().In(loc)
	days := int(now.Sub(start).Hours() / 24)
	for i := 0; i <= days; i++ {
		day := time.Date(start.Year(), start.Month(), start.Day(), 12, 0, 0, 0, loc).AddDate(0, 0, i)

		if i%7 == 0 {
			place := places[rnd.Intn(len(places))]
			cents := place.minCOP + rnd.Intn(place.maxCOP-place.minCOP)
			if err := out(day, category, place.name+" supermercado", place.name,
				bancolombia.ID, fmt.Sprint(cents)); err != nil {
				return err
			}
		}

		for _, s := range subs {
			if day.Day() != s.day {
				continue
			}
			if err := out(day, subsCat, s.name+" suscripción", s.name, s.acc, s.amount); err != nil {
				return err
			}
		}

		if day.Day() == 25 {
			salary, err := money.Parse("5400000")
			if err != nil {
				return err
			}
			if err := add(finance.NewTransaction{
				AccountID:   bancolombia.ID,
				CategoryID:  &salaryCat,
				Description: "Nómina mensual",
				Merchant:    strPtr("Acme Media SAS"),
				Amount:      salary,
				Direction:   "in",
				Currency:    "COP",
				Status:      "posted",
				OccurredAt:  &day,
			}); err != nil {
				return err
			}
		}
	}

	fmt.Printf("seeding finished: %s / %s created user, 2 accounts, %d transactions\n", user.Email, demoPassword, created)
	return nil
}

func strPtr(s string) *string { return &s }
