DROP TABLE IF EXISTS driver_tasks;

ALTER TABLE orders
  DROP COLUMN pickup_schedule,
  DROP COLUMN auto_confirm_at;
