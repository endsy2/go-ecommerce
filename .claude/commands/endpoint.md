Create a new API endpoint for: $ARGUMENTS

Follow the layering rules in backend/CLAUDE.md. Steps:
1. Show me the plan first: route, request DTO, response DTO, service method,
   repository method, and any migration needed. Stop and wait for approval.
2. Migration (if schema changes)
3. Model / DTO
4. Repository method + test
5. Service method + test
6. Handler + route registration
7. Run go test ./... and go vet ./...
8. Give me a curl command to test it manually