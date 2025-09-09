-- +goose Up
-- +goose StatementBegin

-- Create comprehensive table with all PostgreSQL field types
CREATE TABLE comprehensive_example (
    -- Primary key variations
    id BIGSERIAL PRIMARY KEY,
    uuid_id UUID DEFAULT gen_random_uuid() UNIQUE,
    
    -- Numeric types with various constraints
    small_int SMALLINT NOT NULL DEFAULT 0 CHECK (small_int >= 0),
    regular_int INTEGER DEFAULT 42,
    big_int BIGINT NOT NULL,
    decimal_precise DECIMAL(10,2) DEFAULT 0.00,
    numeric_field NUMERIC(15,4) NOT NULL DEFAULT 1000.0000,
    real_float REAL DEFAULT 3.14159,
    double_float DOUBLE PRECISION NOT NULL DEFAULT 2.71828,
    
    -- Serial types
    small_serial SMALLSERIAL NOT NULL,
    big_serial BIGSERIAL UNIQUE,
    
    -- Character/String types
    fixed_char CHAR(10) DEFAULT 'DEFAULT   ',
    variable_char VARCHAR(255) NOT NULL DEFAULT 'required field',
    unlimited_text TEXT,
    text_with_default TEXT DEFAULT 'Some default text content',
    text_not_null TEXT NOT NULL,
    
    -- Boolean variations
    is_active BOOLEAN DEFAULT TRUE,
    is_verified BOOLEAN NOT NULL DEFAULT FALSE,
    nullable_flag BOOLEAN,
    
    -- Date and Time types
    created_date DATE DEFAULT CURRENT_DATE,
    birth_date DATE NOT NULL,
    exact_time TIME DEFAULT CURRENT_TIME,
    time_with_zone TIMETZ DEFAULT CURRENT_TIME,
    created_timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
    timestamp_with_zone TIMESTAMPTZ DEFAULT NOW(),
    
    -- Interval type
    duration_interval INTERVAL DEFAULT '1 day',
    work_hours INTERVAL NOT NULL DEFAULT '8 hours',
    
    -- Binary data
    file_data BYTEA,
    required_binary BYTEA NOT NULL DEFAULT '\x',
    
    -- Network types
    ip_address INET,
    ip_network CIDR,
    mac_address MACADDR,
    mac8_address MACADDR8,
    
    -- Geometric types
    point_location POINT,
    line_segment LSEG,
    rectangular_box BOX,
    path_data PATH,
    polygon_shape POLYGON,
    circle_area CIRCLE,
    
    -- JSON types
    json_data JSON,
    jsonb_data JSONB DEFAULT '{}',
    jsonb_not_null JSONB NOT NULL DEFAULT '{"initialized": true}',
    
    -- Array types
    integer_array INTEGER[] DEFAULT ARRAY[1,2,3],
    text_array TEXT[] NOT NULL DEFAULT ARRAY['default', 'values'],
    multidim_array INTEGER[][] DEFAULT '{{1,2},{3,4}}',
    
    -- Range types
    int_range INT4RANGE,
    bigint_range INT8RANGE,
    numeric_range NUMRANGE DEFAULT '[1.0, 100.0)',
    timestamp_range TSRANGE,
    timestamptz_range TSTZRANGE DEFAULT '[2024-01-01, 2024-12-31)',
    date_range DATERANGE NOT NULL DEFAULT '[2024-01-01, 2024-12-31]',
    
    -- UUID type
    reference_uuid UUID,
    required_uuid UUID NOT NULL DEFAULT gen_random_uuid(),
    
    -- Money type
    price MONEY DEFAULT '$0.00',
    salary MONEY NOT NULL,
    
    -- Bit string types
    bit_field BIT(8) DEFAULT B'00000000',
    variable_bits VARBIT(16),
    
    -- XML type
    xml_content XML,
    
    -- Status as varchar instead of enum
    current_status VARCHAR(20) DEFAULT 'pending' CHECK (current_status IN ('active', 'inactive', 'pending', 'deleted')),
    user_mood VARCHAR(10) NOT NULL DEFAULT 'neutral' CHECK (user_mood IN ('happy', 'sad', 'neutral')),
    
    -- Text search types
    search_vector TSVECTOR,
    search_query TSQUERY,
    
    -- Special constraint combinations
    percentage NUMERIC(5,2) CHECK (percentage >= 0.00 AND percentage <= 100.00) DEFAULT 0.00,
    email VARCHAR(255) UNIQUE CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'),
    age INTEGER CHECK (age >= 0 AND age <= 150),
    
    -- Fields with multiple constraints
    username VARCHAR(50) NOT NULL UNIQUE CHECK (length(username) >= 3),
    slug VARCHAR(100) NOT NULL UNIQUE DEFAULT 'default-slug',
    
    -- Nullable fields with various defaults
    optional_date DATE,
    optional_number INTEGER,
    optional_text TEXT,
    
    -- Fields that reference other constraints
    parent_id BIGINT REFERENCES comprehensive_example(id) ON DELETE CASCADE,
    
    -- Timestamps for auditing
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    
    -- Version control
    version INTEGER NOT NULL DEFAULT 1
    
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Drop table
DROP TABLE IF EXISTS comprehensive_example;

-- +goose StatementEnd
