ALTER TABLE customer_addresses
  DROP COLUMN province_id,
  DROP COLUMN city_id,
  DROP COLUMN district_id,
  ADD COLUMN province TEXT NOT NULL,
  ADD COLUMN city TEXT NOT NULL,
  ADD COLUMN district TEXT NOT NULL;
