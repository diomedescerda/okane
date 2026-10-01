-- +goose Up

-- Imported transactions are in Colombian pesos like everything else now.
ALTER TABLE transactions ALTER COLUMN currency SET DEFAULT 'COP';
UPDATE transactions SET currency = 'COP' WHERE currency = 'MXN';

-- +goose Down

ALTER TABLE transactions ALTER COLUMN currency SET DEFAULT 'MXN';
UPDATE transactions SET currency = 'MXN' WHERE currency = 'COP';
