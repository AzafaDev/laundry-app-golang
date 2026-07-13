ALTER TABLE customer_addresses
  DROP COLUMN province,
  DROP COLUMN city,
  DROP COLUMN district,
  ADD COLUMN province_id INTEGER NOT NULL REFERENCES provinces(id),
  ADD COLUMN city_id INTEGER NOT NULL REFERENCES cities(id),
  ADD COLUMN district_id INTEGER NOT NULL REFERENCES districts(id);
