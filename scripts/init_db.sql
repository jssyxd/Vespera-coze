-- Vespera Database Initialization Script
-- Run this in Supabase SQL Editor

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Ethereum Mainnet Table
CREATE TABLE IF NOT EXISTS ethereum (
    address VARCHAR(42) PRIMARY KEY,
    contract TEXT NOT NULL,
    abi JSONB,
    balance VARCHAR(50) DEFAULT '0.000000',
    isopensource BOOLEAN DEFAULT FALSE,
    isproxy BOOLEAN DEFAULT FALSE,
    implementation VARCHAR(42),
    createtime TIMESTAMP NOT NULL DEFAULT NOW(),
    createblock BIGINT NOT NULL DEFAULT 0,
    txlast TIMESTAMP NOT NULL DEFAULT NOW(),
    isdecompiled BOOLEAN DEFAULT FALSE,
    dedcode TEXT,
    scan_result JSONB,
    scan_time TIMESTAMP
);

-- BSC Table
CREATE TABLE IF NOT EXISTS bsc (
    address VARCHAR(42) PRIMARY KEY,
    contract TEXT NOT NULL,
    abi JSONB,
    balance VARCHAR(50) DEFAULT '0.000000',
    isopensource BOOLEAN DEFAULT FALSE,
    isproxy BOOLEAN DEFAULT FALSE,
    implementation VARCHAR(42),
    createtime TIMESTAMP NOT NULL DEFAULT NOW(),
    createblock BIGINT NOT NULL DEFAULT 0,
    txlast TIMESTAMP NOT NULL DEFAULT NOW(),
    isdecompiled BOOLEAN DEFAULT FALSE,
    dedcode TEXT,
    scan_result JSONB,
    scan_time TIMESTAMP
);

-- Polygon Table
CREATE TABLE IF NOT EXISTS polygon (
    address VARCHAR(42) PRIMARY KEY,
    contract TEXT NOT NULL,
    abi JSONB,
    balance VARCHAR(50) DEFAULT '0.000000',
    isopensource BOOLEAN DEFAULT FALSE,
    isproxy BOOLEAN DEFAULT FALSE,
    implementation VARCHAR(42),
    createtime TIMESTAMP NOT NULL DEFAULT NOW(),
    createblock BIGINT NOT NULL DEFAULT 0,
    txlast TIMESTAMP NOT NULL DEFAULT NOW(),
    isdecompiled BOOLEAN DEFAULT FALSE,
    dedcode TEXT,
    scan_result JSONB,
    scan_time TIMESTAMP
);

-- Arbitrum Table
CREATE TABLE IF NOT EXISTS arbitrum (
    address VARCHAR(42) PRIMARY KEY,
    contract TEXT NOT NULL,
    abi JSONB,
    balance VARCHAR(50) DEFAULT '0.000000',
    isopensource BOOLEAN DEFAULT FALSE,
    isproxy BOOLEAN DEFAULT FALSE,
    implementation VARCHAR(42),
    createtime TIMESTAMP NOT NULL DEFAULT NOW(),
    createblock BIGINT NOT NULL DEFAULT 0,
    txlast TIMESTAMP NOT NULL DEFAULT NOW(),
    isdecompiled BOOLEAN DEFAULT FALSE,
    dedcode TEXT,
    scan_result JSONB,
    scan_time TIMESTAMP
);

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_ethereum_createtime ON ethereum(createtime DESC);
CREATE INDEX IF NOT EXISTS idx_ethereum_createblock ON ethereum(createblock DESC);
CREATE INDEX IF NOT EXISTS idx_ethereum_isopensource ON ethereum(isopensource);
CREATE INDEX IF NOT EXISTS idx_ethereum_scantime ON ethereum(scan_time);

CREATE INDEX IF NOT EXISTS idx_bsc_createtime ON bsc(createtime DESC);
CREATE INDEX IF NOT EXISTS idx_bsc_createblock ON bsc(createblock DESC);

CREATE INDEX IF NOT EXISTS idx_polygon_createtime ON polygon(createtime DESC);
CREATE INDEX IF NOT EXISTS idx_polygon_createblock ON polygon(createblock DESC);

CREATE INDEX IF NOT EXISTS idx_arbitrum_createtime ON arbitrum(createtime DESC);
CREATE INDEX IF NOT EXISTS idx_arbitrum_createblock ON arbitrum(createblock DESC);

-- GIN index for JSONB scan_result (for advanced querying)
CREATE INDEX IF NOT EXISTS idx_ethereum_scanresult ON ethereum USING GIN (scan_result);

-- Verification query
SELECT 'Tables created successfully' AS status;
SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename IN ('ethereum', 'bsc', 'polygon', 'arbitrum');
