# Changelog

## [0.2.0](https://github.com/divmora/periscope/compare/v0.1.0...v0.2.0) (2026-10-15)


### Features

* **auth:** implement project-scoped RBAC and team membership management ([3cd583d](https://github.com/divmora/periscope/commit/3cd583dba260a6790f1ca47b6cc3a438e71e8509))
* **collector:** implement storage drivers, telescope drainer, parquet encoder, and sync CLI ([54b77ca](https://github.com/divmora/periscope/commit/54b77cab62be6b7d8d11b5eca282d42c25f17aec))
* **collector:** namespace Hive partitions with source (e.g. source=laravel_telescope) ([9406c66](https://github.com/divmora/periscope/commit/9406c66a2792c09e912d3a466e7550d59a6c972a))
* **das:** add dual direct S3 DuckDB and Athena query engines for Aurora DAS ([61976e5](https://github.com/divmora/periscope/commit/61976e56bdca09d87bdabbdd82692ac67180daed))
* **das:** support per-source AWS Profile, Static Credentials, and IAM Role Assume ([c2e217b](https://github.com/divmora/periscope/commit/c2e217b942e14dc954f0bc7fdff29f67d9f9fd7a))
* implement UI overhaul and apply Periscope branding rename ([3edf681](https://github.com/divmora/periscope/commit/3edf681a19c1f3a34965f145c2521a968448c8aa))
* introduce multi-source architecture, DuckDB SQL Studio, and retention lifecycle ([c2d842b](https://github.com/divmora/periscope/commit/c2d842b18f440649543d77c5f3dbd30ce218cccf))
* **rbac:** implement project teams, source-level access control, and decoupled project creation ([b60ba39](https://github.com/divmora/periscope/commit/b60ba39abe1d3d74449c53404e264fca3141b0d1))
* **ui:** add LogsPage component and configure RustFS in docker-compose ([ab59bdc](https://github.com/divmora/periscope/commit/ab59bdc78e66d136a663450344c575951b499206))
* **ui:** bind engine to selected DAS source and remove manual toggle ([0259e98](https://github.com/divmora/periscope/commit/0259e984f66265dd70768208be9faa3662f8ecbd))
* **ui:** build project creation UI flow ([f15b88e](https://github.com/divmora/periscope/commit/f15b88e4ec478d01c1e43f8a628d9e525c4998ab))
* **ui:** group Telescope telemetry items as a collapsible submenu ([73bd19c](https://github.com/divmora/periscope/commit/73bd19c85a73b4c2c76b6a91ddf0bbb1d32a7821))
* **ui:** redesign DAS view with balanced header, time range picker, active users KPI, and overflow-safe table ([242845b](https://github.com/divmora/periscope/commit/242845b9bb754892260e55c8e96136f9255d2929))


### Bug Fixes

* **lint:** resolve golangci-lint errors across handlers, storage, and cmd ([48635c8](https://github.com/divmora/periscope/commit/48635c8795c7f4fd2c0ea3a475a4eb50e3a9c05c))
* **olap:** enable DuckDB execution in container and map Aurora DAS parquet schema ([f1b38b6](https://github.com/divmora/periscope/commit/f1b38b604a6c8843e11a5691e9b64c0c68859331))
