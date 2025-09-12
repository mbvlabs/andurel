-- +goose Up
-- +goose StatementBegin
CREATE TABLE products (
    id TEXT PRIMARY KEY,
    
    int_field INTEGER,
    integer_field INTEGER,
    tinyint_field TINYINT,
    smallint_field SMALLINT,
    mediumint_field MEDIUMINT,
    bigint_field BIGINT,
    unsigned_bigint_field UNSIGNED BIG INT,
    int2_field INT2,
    int8_field INT8,
    
    boolean_field BOOLEAN,
    bool_field BOOL,
    
    character_field CHARACTER(20),
    varchar_field VARCHAR(255),
    varying_character_field VARYING CHARACTER(255),
    nchar_field NCHAR(55),
    native_character_field NATIVE CHARACTER(70),
    nvarchar_field NVARCHAR(100),
    text_field TEXT,
    clob_field CLOB,
    
    char_field CHAR(10),
    
    real_field REAL,
    double_field DOUBLE,
    double_precision_field DOUBLE PRECISION,
    float_field FLOAT,
    
    numeric_field NUMERIC,
    decimal_field DECIMAL(10,5),
    dec_field DECIMAL(10,5),
    
    blob_field BLOB,
    
    date_as_text DATE,           
    datetime_as_text DATETIME,   
    timestamp_field TIMESTAMP,   
    time_field TIME,             
    
    required_text TEXT NOT NULL,
    required_int INTEGER NOT NULL,
    
    default_text TEXT DEFAULT 'default_value',
    default_int INTEGER DEFAULT 0,
    default_real REAL DEFAULT 0.0,
    default_bool BOOLEAN DEFAULT FALSE,
    default_timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    positive_int INTEGER CHECK (positive_int > 0),
    email_text TEXT CHECK (email_text LIKE '%@%.%'),
    
    unique_text TEXT UNIQUE,
    unique_int INTEGER UNIQUE
);
-- +goose StatementEnd
