 # ADR 0001 - Autenticação, Sessões e CSRF                                                       
                                                                                                  
  - Data: 2026-03-03                                                                              
  - Status: Aceito                                                                                
  - Contexto:                                                                                     
    Aplicação monolítica Go, baixa carga inicial, objetivo de estudo com foco em escalabilidade   
  futura (HA/distribuição).                                                                       
                                                                                                  
  ## Decisão                                                                                      
  1. Sessões stateful em Redis.                                                                   
  2. Cookie de sessão HttpOnly com ID aleatório (sem dados sensíveis).                            
  3. Sessões longas (30 dias), com expiração em Redis.                                            
  4. Senhas com bcrypt.                                                                           
  5. Proteção CSRF com Synchronizer Token Pattern.                                                
  6. Logout invalida sessão no Redis e remove cookie.                                             
  7. Configuração de cookie:                                                                      
     - HttpOnly=true                                                                              
     - Secure=true (produção)                                                                     
     - SameSite=Lax                                                                               
     - Path=/                                                                                     
                                                                                                  
  ## Consequências                                                                                
  - Prós:                                                                                         
    - Escalável horizontalmente.                                                                  
    - Revogação/invalidade de sessão simples.                                                     
    - Menor exposição de dados no cliente.                                                        
  - Contras:                                                                                      
    - Dependência operacional de Redis.                                                           
    - Complexidade maior que sessão em banco local.                                               
                                                                                                  
  ## Alternativas consideradas                                                                    
  1. Sessão em banco relacional.                                                                  
  2. Cookie assinado/encriptado (stateless).                                                      
  3. JWT em cookie HttpOnly.                                                                      
                                                                                                  
  ## Pendências                                                                                   
  1. Expiração deslizante vs fixa.                                                                
  2. Rotação de sessão após login.                                                                
  3. Política de múltiplas sessões por usuário.                                                   
  4. Política de lockout por tentativas.                                                          
  5. Escopo de auditoria e retenção de logs.  