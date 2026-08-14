-- Modify "gb_config_releases" table
ALTER TABLE `gb_config_releases` ADD COLUMN `reason` varchar(1000) NOT NULL DEFAULT "" AFTER `descriptions`;
