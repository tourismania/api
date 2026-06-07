CREATE UNIQUE INDEX cities_uniq_name_state_country
    ON cities (lower(name), COALESCE(lower(state), ''), country_iso2);
