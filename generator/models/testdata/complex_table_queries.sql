-- name: QueryComprehensiveExampleByID :one
select * from comprehensive_example where id=$1;

-- name: QueryComprehensiveExamples :many
select * from comprehensive_example;

-- name: QueryAllComprehensiveExamples :many
select * from comprehensive_example;

-- name: InsertComprehensiveExample :one
insert into
    comprehensive_example (id, uuid_id, small_int, regular_int, big_int, decimal_precise, numeric_field, real_float, double_float, small_serial, big_serial, fixed_char, variable_char, unlimited_text, text_with_default, text_not_null, is_active, is_verified, nullable_flag, created_date, birth_date, exact_time, time_with_zone, created_timestamp, updated_timestamp, timestamp_with_zone, duration_interval, work_hours, file_data, required_binary, ip_address, ip_network, mac_address, mac8_address, point_location, line_segment, rectangular_box, path_data, polygon_shape, circle_area, json_data, jsonb_data, jsonb_not_null, integer_array, text_array, multidim_array, int_range, bigint_range, numeric_range)
values
    ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36, $37, $38, $39, $40, $41, $42, $43, $44, $45, $46, $47, $48, $49)
returning *;

-- name: UpdateComprehensiveExample :one
update comprehensive_example
    set uuid_id=$2, small_int=$3, regular_int=$4, big_int=$5, decimal_precise=$6, numeric_field=$7, real_float=$8, double_float=$9, small_serial=$10, big_serial=$11, fixed_char=$12, variable_char=$13, unlimited_text=$14, text_with_default=$15, text_not_null=$16, is_active=$17, is_verified=$18, nullable_flag=$19, created_date=$20, birth_date=$21, exact_time=$22, time_with_zone=$23, created_timestamp=$24, updated_timestamp=$25, timestamp_with_zone=$26, duration_interval=$27, work_hours=$28, file_data=$29, required_binary=$30, ip_address=$31, ip_network=$32, mac_address=$33, mac8_address=$34, point_location=$35, line_segment=$36, rectangular_box=$37, path_data=$38, polygon_shape=$39, circle_area=$40, json_data=$41, jsonb_data=$42, jsonb_not_null=$43, integer_array=$44, text_array=$45, multidim_array=$46, int_range=$47, bigint_range=$48, numeric_range=$49
where id = $1
returning *;

-- name: DeleteComprehensiveExample :exec
delete from comprehensive_example where id=$1;

-- name: QueryPaginatedComprehensiveExamples :many
select * from comprehensive_example 
order by created_at desc 
limit sqlc.arg('limit')::bigint offset sqlc.arg('offset')::bigint;

-- name: CountComprehensiveExamples :one
select count(*) from comprehensive_example;

