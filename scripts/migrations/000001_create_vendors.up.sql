CREATE TABLE vendors (
    id            SERIAL PRIMARY KEY,
    vendor_code   VARCHAR(64)  UNIQUE NOT NULL,
    company_name  VARCHAR(256) NOT NULL,
    netsuite_id   VARCHAR(64)  NOT NULL,
    contact_email VARCHAR(256),
    contact_phone VARCHAR(64),
    status        VARCHAR(32)  NOT NULL DEFAULT 'PENDING',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
