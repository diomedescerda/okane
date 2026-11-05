import assert from "node:assert/strict";
import test from "node:test";

import {
  DICTIONARIES,
  SUPPORTED_LANGUAGES,
  normalizeLanguage,
  resolveInitialLanguage,
  translate,
  translateAccountType,
  translateApiError,
  translateCategoryName,
  translateInstitutionType,
  translateMatchField,
  translateMatchType,
} from "../src/lib/i18n.ts";
import {
  formatDate,
  formatMoney,
  formatMonth,
  getFormatLanguage,
  setFormatLanguage,
} from "../src/lib/format.ts";

test("normalizeLanguage accepts en, es, ja and locale variants", () => {
  assert.equal(normalizeLanguage("en"), "en");
  assert.equal(normalizeLanguage("en-US"), "en");
  assert.equal(normalizeLanguage("es"), "es");
  assert.equal(normalizeLanguage("es-CO"), "es");
  assert.equal(normalizeLanguage("ja"), "ja");
  assert.equal(normalizeLanguage("ja-JP"), "ja");
  assert.equal(normalizeLanguage("fr"), null);
  assert.equal(normalizeLanguage(""), null);
  assert.equal(normalizeLanguage(null), null);
});

test("resolveInitialLanguage prioritizes stored language, then browser, then en", () => {
  assert.equal(
    resolveInitialLanguage({
      stored: "ja",
      browserLanguages: ["en-US"],
    }),
    "ja",
  );
  assert.equal(
    resolveInitialLanguage({
      stored: "es",
      browserLanguages: ["en-US"],
    }),
    "es",
  );
  assert.equal(
    resolveInitialLanguage({
      stored: null,
      browserLanguages: ["fr-FR", "ja-JP", "en-US"],
    }),
    "ja",
  );
  assert.equal(
    resolveInitialLanguage({
      stored: null,
      browserLanguages: ["de-DE"],
    }),
    "en",
  );
});

test("translate interpolates parameters in en, es, and ja and all dictionaries are complete", () => {
  assert.equal(translate("en", "layout.welcomeBackName", { name: "Kenma" }), "Welcome back, Kenma");
  assert.equal(
    translate("es", "layout.welcomeBackName", { name: "Kenma" }),
    "Hola de nuevo, Kenma",
  );
  assert.equal(
    translate("ja", "layout.welcomeBackName", { name: "Kenma" }),
    "おかえりなさい、Kenmaさん",
  );

  const enKeys = Object.keys(DICTIONARIES.en);
  for (const lang of SUPPORTED_LANGUAGES) {
    assert.deepEqual(Object.keys(DICTIONARIES[lang]).sort(), [...enKeys].sort());
    for (const key of enKeys) {
      assert.ok(
        DICTIONARIES[lang][key].length > 0,
        `Expected non-empty translation for ${lang}:${key}`,
      );
    }
  }
});

test("translateCategoryName localizes system categories by slug or English fallback", () => {
  assert.equal(translateCategoryName("en", "groceries", "Groceries"), "Groceries");
  assert.equal(translateCategoryName("es", "groceries", "Groceries"), "Mercado y supermercado");
  assert.equal(translateCategoryName("ja", "groceries", "Groceries"), "食料品");

  // Fallback when slug is omitted but English name is known
  assert.equal(translateCategoryName("es", undefined, "Food & Drinks"), "Comida y bebidas");
  assert.equal(translateCategoryName("ja", undefined, "Food & Drinks"), "食費・飲食");

  // Custom user category stays untouched
  assert.equal(translateCategoryName("ja", "my-custom-cat", "My Custom Cat"), "My Custom Cat");
});

test("enum translators localize account, institution, match type, and match field", () => {
  assert.equal(translateAccountType("es", "checking"), "Cuenta corriente");
  assert.equal(translateAccountType("ja", "savings"), "貯蓄預金");
  assert.equal(translateInstitutionType("es", "wallet"), "Billetera");
  assert.equal(translateInstitutionType("ja", "bank"), "銀行");
  assert.equal(translateMatchType("es", "contains"), "contiene");
  assert.equal(translateMatchType("ja", "contains"), "を含む");
  assert.equal(translateMatchField("es", "merchant"), "Comercio");
  assert.equal(translateMatchField("ja", "description"), "内容");
});

test("translateApiError localizes known backend error messages", () => {
  assert.equal(
    translateApiError("es", new Error("invalid email or password")),
    "Correo electrónico o contraseña incorrectos",
  );
  assert.equal(
    translateApiError("ja", new Error("an account with this email already exists")),
    "このメールアドレスのアカウントは既に存在します",
  );
});

test("format utilities adapt to active language", () => {
  setFormatLanguage("en");
  assert.equal(getFormatLanguage(), "en");
  const enMonth = formatMonth("2026-05");
  assert.match(enMonth, /May/i);

  setFormatLanguage("es");
  assert.equal(getFormatLanguage(), "es");
  const esMonth = formatMonth("2026-05");
  assert.match(esMonth, /may/i);

  setFormatLanguage("ja");
  assert.equal(getFormatLanguage(), "ja");
  const jaDate = formatDate("2026-05-15T12:00:00Z");
  assert.ok(jaDate.length > 0);
  const jaMoney = formatMoney(125000, "COP");
  assert.match(jaMoney, /125,000/);

  setFormatLanguage("en");
});
