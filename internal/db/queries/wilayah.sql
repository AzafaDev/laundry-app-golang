-- name: ListProvinces :many
SELECT * FROM provinces ORDER BY name;

-- name: ListCitiesByProvince :many
SELECT * FROM cities WHERE province_id = $1 ORDER BY name;

-- name: ListDistrictsByCity :many
SELECT * FROM districts WHERE city_id = $1 ORDER BY name;
