.PHONY: eval eval-mock seed-vector reset-vector

seed-vector:
	cd server && go run ./cmd/seed-demo-vector/

reset-vector:
	cd server && go run ./cmd/reset-demo-vector/

eval-mock:
	cd server && go test -count=1 ./evals/ -run 'TestVibeSenseEvalGate|TestKeyLeakGuard|TestVibeSenseVectorRegressionGate|TestCompareBaseline|TestCatalogVectorRetrievalIntegration' -write-reports

eval:
	@test -n "$$GEMINI_API_KEY" || (echo "GEMINI_API_KEY is not set" >&2; exit 1)
	@mkdir -p artifacts/live-evals
	./scripts/run-live-evals.sh
	./scripts/check-artifact-secrets.sh
	python3 ./scripts/print-eval-threshold-table.py
