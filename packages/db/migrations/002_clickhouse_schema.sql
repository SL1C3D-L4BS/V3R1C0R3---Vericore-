-- 002_clickhouse_schema.sql
-- Warm tier: audit_log for MMR leaves exported from LibSQL (Hot).
-- Partitioned by month for Tombstone Proof and Move-to-Cold.
--
-- Storage policy (define on the server; not in this file):
--   STORAGE POLICY tiered:
--     hot  -> default disk (local/SSD)
--     cold -> S3 volume (e.g. s3_disk)
-- Example server config:
--   <storage_configuration>
--     <disks>
--       <default><path>/var/lib/clickhouse/</path></default>
--       <s3_disk type="s3">
--         <endpoint>https://your-bucket.s3.region.amazonaws.com/cold/</endpoint>
--         <access_key_id>...</access_key_id>
--         <secret_access_key>...</secret_access_key>
--       </s3_disk>
--     </disks>
--     <policies>
--       <tiered>
--         <volumes>
--           <hot><disk>default</disk></hot>
--           <cold><disk>s3_disk</disk></cold>
--         </volumes>
--       </tiered>
--     </policies>
--   </storage_configuration>
-- Then: SETTINGS storage_policy = 'tiered'

CREATE TABLE IF NOT EXISTS audit_log
(
    id          String,
    mmr_index   UInt64,
    event_id    String,
    hash        String,
    tenant_id   String DEFAULT 'default',
    ingested_at DateTime64(3) DEFAULT now64(3)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(ingested_at)
ORDER BY (tenant_id, ingested_at, mmr_index)
SETTINGS storage_policy = 'tiered';

-- Partition id is YYYYMM (e.g. '202603'). After tombstone is saved:
-- ALTER TABLE audit_log MOVE PARTITION '<partitionMonth>' TO VOLUME 'cold';
