alter table login_log add index user_status (user_id, succeeded );
alter table login_log add index ip_status (ip, succeeded );
