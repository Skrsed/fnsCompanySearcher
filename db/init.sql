DROP TABLE IF EXISTS Companies;
CREATE TABLE Companies (
    ogrn INTEGER UNIQUE,
    contacts VARCHAR,
    finances VARCHAR,
    inn VARCHAR,
    ceo VARCHR,
    json_data JSONB
);

DROP TABLE IF EXISTS Cached;
CREATE TABLE Cached (
    ogrn INTEGER UNIQUE
)