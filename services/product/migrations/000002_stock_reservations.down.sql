-- Migration 000002 (down): drop reservation tables in FK-dependency order.
DROP TABLE IF EXISTS reservation_items;
DROP TABLE IF EXISTS stock_reservations;
