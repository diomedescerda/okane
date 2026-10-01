-- +goose Up

-- Colombian banks and wallets. Logos are bundled with the web app under
-- /institutions/; rows without a logo fall back to an icon or initials.
INSERT INTO institutions
    (slug, name, display_name, country_code, type, website, primary_color, logo_url, icon, sort_order, is_system)
VALUES
    ('bancolombia',         'bancolombia',         'Bancolombia',           'CO', 'bank',   'https://www.bancolombia.com',              '#FDDA24', '/institutions/bancolombia.png',         '🏦',  10, true),
    ('banco-de-bogota',     'banco_de_bogota',     'Banco de Bogotá',       'CO', 'bank',   'https://www.bancodebogota.com',            '#0033A0', '/institutions/banco-de-bogota.png',     '🏦',  11, true),
    ('davivienda',          'davivienda',          'Davivienda',            'CO', 'bank',   'https://www.davivienda.com',               '#E30613', '/institutions/davivienda.png',          '🏦',  12, true),
    ('bbva-colombia',       'bbva_colombia',       'BBVA Colombia',         'CO', 'bank',   'https://www.bbva.com.co',                  '#004481', '/institutions/bbva-colombia.png',       '🏦',  13, true),
    ('banco-de-occidente',  'banco_de_occidente',  'Banco de Occidente',    'CO', 'bank',   'https://www.bancodeoccidente.com.co',      '#004B93', '/institutions/banco-de-occidente.png',  '🏦',  14, true),
    ('banco-popular',       'banco_popular',       'Banco Popular',         'CO', 'bank',   'https://www.bancopopular.com.co',          '#00693E', '/institutions/banco-popular.png',       '🏦',  15, true),
    ('scotiabank-colpatria','scotiabank_colpatria','Scotiabank Colpatria',  'CO', 'bank',   'https://www.scotiabankcolpatria.com',      '#EC111A', '/institutions/scotiabank-colpatria.png','🏦',  16, true),
    ('banco-av-villas',     'banco_av_villas',     'Banco AV Villas',       'CO', 'bank',   'https://www.avvillas.com.co',              '#003DA5', '/institutions/banco-av-villas.png',     '🏦',  17, true),
    ('banco-caja-social',   'banco_caja_social',   'Banco Caja Social',     'CO', 'bank',   'https://www.bancocajasocial.com',          '#005CA9', '/institutions/banco-caja-social.png',   '🏦',  18, true),
    ('banco-agrario',       'banco_agrario',       'Banco Agrario',         'CO', 'bank',   'https://www.bancoagrario.gov.co',          '#006B3F', '/institutions/banco-agrario.png',       '🏦',  19, true),
    ('itau-colombia',       'itau_colombia',       'Itaú Colombia',         'CO', 'bank',   'https://www.itau.co',                      '#EC7000', '/institutions/itau-colombia.png',       '🏦',  20, true),
    ('banco-falabella',     'banco_falabella',     'Banco Falabella',       'CO', 'bank',   'https://www.bancofalabella.com.co',        '#008542', '/institutions/banco-falabella.png',     '🏦',  21, true),
    ('banco-pichincha',     'banco_pichincha',     'Banco Pichincha',       'CO', 'bank',   'https://www.bancopichincha.com.co',        '#FFD100', '/institutions/banco-pichincha.png',     '🏦',  22, true),
    ('banco-gnb-sudameris', 'banco_gnb_sudameris', 'Banco GNB Sudameris',   'CO', 'bank',   'https://www.gnbsudameris.com.co',          '#007A33', '',                                      '🏦',  23, true),
    ('lulo-bank',           'lulo_bank',           'Lulo Bank',             'CO', 'bank',   'https://www.lulobank.com',                 '#C4D600', '/institutions/lulo-bank.png',           '🏦',  30, true),
    ('nu-colombia',         'nu_colombia',         'Nu Colombia',           'CO', 'wallet', 'https://www.nu.com.co',                    '#820AD1', '/institutions/nu-colombia.png',         '💳',  31, true),
    ('nequi',               'nequi',               'Nequi',                 'CO', 'wallet', 'https://www.nequi.com',                    '#5C2D91', '/institutions/nequi.png',               '📲',  32, true),
    ('daviplata',           'daviplata',           'Daviplata',             'CO', 'wallet', 'https://www.daviplata.com',                '#E30613', '/institutions/daviplata.png',           '📲',  33, true),
    ('rappipay',            'rappipay',            'RappiPay',              'CO', 'wallet', 'https://www.rappipay.com',                 '#FF441F', '/institutions/rappipay.png',            '🛵',  34, true);

-- +goose Down

DELETE FROM institutions WHERE is_system = true;
