-- name: GetUserById :one                                      
  SELECT id, email, password_hash, password_algo, is_admin,      
  created_at                                                     
  FROM users                                                     
  WHERE id = $1;  

-- name: GetUserByEmail :one
  SELECT id, email, password_hash, password_algo, is_admin,      
  created_at                                                     
  FROM users                                                     
  WHERE email = $1;     

