# acb-enrichment Deployment Progress

## Status: In Progress

## Date: 2024-06-04

## Approach
Triggering CI build via git push webhook to `acb-images-build` WorkflowTemplate which includes enrichment image build.

## Steps
1. [x] Verify `acb-images-build` template includes enrichment
2. [ ] Trigger webhook by pushing to ai-code-battle
3. [ ] Monitor workflow completion
4. [ ] Get image SHA from Docker Hub
5. [ ] Update deployment manifest with real SHA
6. [ ] Push to declarative-config
