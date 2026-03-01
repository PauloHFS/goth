## Description
<!-- Describe the changes made in this pull request. What problem does it solve or what feature does it add? -->

## Related Issue
<!-- Link to the issue(s) this PR addresses. Use keywords like "Fixes", "Closes", or "Resolves" followed by the issue number. -->
Fixes #

## Type of Change
<!-- Mark the appropriate option with an [x] -->
- [ ] Bug fix (non-breaking change that fixes an issue)
- [ ] New feature (non-breaking change that adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to change)
- [ ] Documentation update
- [ ] Code refactoring
- [ ] Performance improvement
- [ ] Security enhancement
- [ ] Database migration
- [ ] Configuration change
- [ ] Other (please describe):

## Testing
<!-- Describe the tests you ran to verify your changes. -->

### Unit Tests
- [ ] All existing unit tests pass
- [ ] New unit tests added for new functionality

### Integration Tests
- [ ] Integration tests pass
- [ ] Database migrations tested (if applicable)

### E2E Tests
- [ ] E2E tests pass (if applicable)
- [ ] Playwright tests verified

### Manual Testing
<!-- List manual testing scenarios if applicable -->
- [ ] Tested locally with `make dev`
- [ ] Verified database migrations with `make migrate-status`
- [ ] Checked security headers
- [ ] Verified CSS generation with `make css`

## Code Quality Checklist
- [ ] My code follows the project's code style
- [ ] I have run `make lint` and addressed all linting issues
- [ ] I have run `make sec` and addressed security concerns
- [ ] I have run `make generate` and committed generated files
- [ ] My changes generate no new warnings
- [ ] I have updated the documentation accordingly
- [ ] I have verified that `make test` passes locally

## Security Considerations
<!-- Describe any security implications of your changes -->
- [ ] No sensitive data exposed in logs
- [ ] Input validation implemented where needed
- [ ] SQL injection prevention verified
- [ ] XSS prevention verified
- [ ] Authentication/Authorization checks in place

## Database Changes
<!-- If your PR includes database changes -->
- [ ] Migration files created and tested
- [ ] Rollback plan defined
- [ ] Data migration tested (if applicable)
- [ ] Indexes added for performance (if needed)

## Screenshots / Recordings
<!-- If applicable, add screenshots or screen recordings to demonstrate the changes -->

<details>
<summary>Before</summary>

<!-- Add screenshot -->

</details>

<details>
<summary>After</summary>

<!-- Add screenshot -->

</details>

## Performance Impact
<!-- Describe any known performance implications -->
- [ ] No performance impact expected
- [ ] Performance improvement expected
- [ ] Performance regression possible (explain below):

## Deployment Notes
<!-- Any special considerations for deployment -->
- [ ] No special deployment steps required
- [ ] Requires database migration before deployment
- [ ] Requires configuration changes
- [ ] Requires environment variable updates
- [ ] Backwards compatible: Yes / No

### Environment Variables (if applicable)
| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
|          |          |         |             |

## Additional Notes
<!-- Any additional information that reviewers should know -->

---

## Reviewer Checklist
- [ ] Code follows project style guidelines
- [ ] Self-review has been performed
- [ ] Tests have been added/updated
- [ ] Documentation has been updated
- [ ] Security implications reviewed
- [ ] Performance impact assessed
