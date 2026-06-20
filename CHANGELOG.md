# Changelog

## [1.0.0](https://github.com/julian-alarcon/DoTheSplit/compare/v0.11.1...v1.0.0) (2026-06-20)


### Features

* **api,frontend:** enable realtime update and unread badge ([782fe8a](https://github.com/julian-alarcon/DoTheSplit/commit/782fe8a1a777cbea2e444f03f12c78ee36a3856a))
* **config:** add SlogLevel helper to parse LOG_LEVEL ([8fc4a70](https://github.com/julian-alarcon/DoTheSplit/commit/8fc4a705c6bc12c4fff70450c99c17ab0ac1e557))
* **frontend, api:** remove custom timezone, use client timezone ([0fb1fac](https://github.com/julian-alarcon/DoTheSplit/commit/0fb1facb1d1b8f0bfb808ad2b6b329fe198f39e5))
* **frontend:** added pwa to frontend ([44908b4](https://github.com/julian-alarcon/DoTheSplit/commit/44908b4e3ac6d31174f353849e0f7031f853abad))
* **frontend:** fix field input verification being shown when not needed ([7a1b2e5](https://github.com/julian-alarcon/DoTheSplit/commit/7a1b2e52b3c3ba5631e9ffb7e544fb15636884c5))
* **frontend:** surface unhandled Vue errors via a global error handler ([399d948](https://github.com/julian-alarcon/DoTheSplit/commit/399d9488828f3fa9cb6625091f38f767f9f12ebf))


### Bug Fixes

* **api,frontend:** avatar icon was not being updated ([0edf4e1](https://github.com/julian-alarcon/DoTheSplit/commit/0edf4e1ec070850750918f4777800c305d502943))
* **api,worker:** honor LOG_LEVEL by wiring it into slog ([65263b9](https://github.com/julian-alarcon/DoTheSplit/commit/65263b90ab08a6c924b10ebade76a5b56fc8025f))
* **api:** satisfy ST1005 by moving member-removal message into the handler ([2e3d055](https://github.com/julian-alarcon/DoTheSplit/commit/2e3d0551221b7ab308ffc1808d387d6dbd3e897f))
* **frontend,api:** remove old cookie auth method and other old astro usage ([613c949](https://github.com/julian-alarcon/DoTheSplit/commit/613c9499255cf5f8e41aaf52cedbfa1ab40d3474))
* **frontend,api:** surface member-removal errors and clarify the non-zero balance message ([ed0b76a](https://github.com/julian-alarcon/DoTheSplit/commit/ed0b76aaf22bfbbd236c7a4e7234ac54622a9d4a))
* **frontend:** add missing preflight resets ([3daf433](https://github.com/julian-alarcon/DoTheSplit/commit/3daf4334e79b00b7a9ae5ca360508458dadef22c))
* **frontend:** added back Inter font ([09607dc](https://github.com/julian-alarcon/DoTheSplit/commit/09607dc3ed362b554c65e8ecb2c0fa3f727bc2bd))
* **frontend:** emit confirm before closing dialogs so action handlers see the target ([3fec905](https://github.com/julian-alarcon/DoTheSplit/commit/3fec9055e6f96db888dd9309e22f23a48dfee3b8))
* **frontend:** expose expense notes in the split details modal ([94c301d](https://github.com/julian-alarcon/DoTheSplit/commit/94c301d57e5ace5ed268b71cfc21b18564e208e5))
* **frontend:** fix full-group CSV import, autofill group name, surface server errors ([13e51d7](https://github.com/julian-alarcon/DoTheSplit/commit/13e51d7c707f65134bb157b5a8ce615b551c8609))
* **frontend:** fix settlements style ([faa24be](https://github.com/julian-alarcon/DoTheSplit/commit/faa24bedc7789f7cdb5c92c650bb817e1602f4e9))
* **frontend:** mark session ready on login to avoid a racy boot refresh ([915a3f6](https://github.com/julian-alarcon/DoTheSplit/commit/915a3f6f780706c184d4c7090f3e4a613272d083))
* **frontend:** missing icons ([bfedb43](https://github.com/julian-alarcon/DoTheSplit/commit/bfedb430a6a188a7ad9389c81d3b4e9a97002b8b))
* **frontend:** restore styles in multiple places ([912452c](https://github.com/julian-alarcon/DoTheSplit/commit/912452c14bb57c381ffd75b5d04c37ab3ebf2698))
* **frontend:** See activity button is hiden when not applicable ([b09a1f4](https://github.com/julian-alarcon/DoTheSplit/commit/b09a1f4690fce305bd327d4caf53f78fcf1f92b0))
* **frontend:** set different title page, restore svg favicon ([64728b1](https://github.com/julian-alarcon/DoTheSplit/commit/64728b1482a518133944564e81e4058fc59fffbb))
* **frontend:** small fixes on vue a11y and hardening, fix position of adding expenses block ([7135a7d](https://github.com/julian-alarcon/DoTheSplit/commit/7135a7d42651f88862915971b269631780110d1b))
* **frontend:** stop e2e smoke flake from a racy post-login refresh ([9a5d1b9](https://github.com/julian-alarcon/DoTheSplit/commit/9a5d1b98ce5f52b94b4d2ae1b15b3216ee0fa18a))
* **frontend:** support autocomplete properly ([18efc1e](https://github.com/julian-alarcon/DoTheSplit/commit/18efc1eec59b2ee7d27061a9375ce77bf4aa6aed))
* **frontend:** use outline calendar icon for date picker trigger ([0374094](https://github.com/julian-alarcon/DoTheSplit/commit/0374094f4a4d54297eab5d2fc6c0446841387f37))
* **frontend:** use solid footer background to stop Firefox Android repaint glitch and dropdown menu not shown on top of input fields ([a4bb3b2](https://github.com/julian-alarcon/DoTheSplit/commit/a4bb3b2efc4c75493d5ad750a92a8f36852e2488))
* **import:** commit CSV imports atomically and fail gracefully on timeout ([3977f79](https://github.com/julian-alarcon/DoTheSplit/commit/3977f7963c8a72df3be0b1ee5da032fbb3516ad3))
* **web:** sync api with existing openapi ([dd235e8](https://github.com/julian-alarcon/DoTheSplit/commit/dd235e8d8d107b2333ceee30d3ac6b9eb8e5677a))


### Performance Improvements

* **build:** strip symbols and trim paths from Docker builds ([3a23b7b](https://github.com/julian-alarcon/DoTheSplit/commit/3a23b7b7e0ee335dc254efa10bdeda03be9572b4))
* **build:** strip symbols and trim paths from local Go builds ([b5e2161](https://github.com/julian-alarcon/DoTheSplit/commit/b5e21617229d8c88c22400fec0f84f7053347faf))


### Miscellaneous Chores

* release 1.0.0 ([4cf18a6](https://github.com/julian-alarcon/DoTheSplit/commit/4cf18a607b3f487b8106abfbf8334fcd976b2627))

## [0.11.1](https://github.com/julian-alarcon/DoTheSplit/compare/v0.11.0...v0.11.1) (2026-06-11)


### Bug Fixes

* **deps:** update module github.com/oapi-codegen/runtime to v1.4.1 ([b4fdcaa](https://github.com/julian-alarcon/DoTheSplit/commit/b4fdcaa8cbfbb5d29b1eb95a3d7bee4512bb46f1))
* **web:** disable Shiki highlighting to silence CSP build warning ([b39a3e7](https://github.com/julian-alarcon/DoTheSplit/commit/b39a3e78afb6dfc1b1470d851083e3393d36fca0))

## [0.11.0](https://github.com/julian-alarcon/DoTheSplit/compare/v0.10.0...v0.11.0) (2026-06-11)


### Features

* **api:** trust only configured proxies for client IP and cap  request body size ([3fc975e](https://github.com/julian-alarcon/DoTheSplit/commit/3fc975e46ab08dac721cd61d071f50f37b014ad2))
* restore soft-deleted expenses and settlements via detail page ([2fe071b](https://github.com/julian-alarcon/DoTheSplit/commit/2fe071bb3a457e1065618de29adb2621fdd3d791))
* **web,api:** provide activity log for all actions in the group ([b50dc41](https://github.com/julian-alarcon/DoTheSplit/commit/b50dc4159829e251b0d6e6a9c259e46f65aab1ea))
* **web:** add note field and date picker to settlement creation form ([80101cb](https://github.com/julian-alarcon/DoTheSplit/commit/80101cbf5b403bf60a443d11bf1b8da02a2921dd))
* **web:** viewer-centric balances and click-to-prefill settle transfers ([4b37ea3](https://github.com/julian-alarcon/DoTheSplit/commit/4b37ea323ffc7ef0baae1481349d2cfb7f76ca64))


### Bug Fixes

* **api:** add read, write, and idle timeouts to the HTTP server ([86b4699](https://github.com/julian-alarcon/DoTheSplit/commit/86b4699e12c98bfbb650cc2be8c5ddafe078a716))
* **api:** anchor omitted expense/settlement dates to noon UTC so same-day items sort consistently ([c666855](https://github.com/julian-alarcon/DoTheSplit/commit/c666855206b4da23b0ed54ba249a12f28e384866))
* **api:** neutralize spreadsheet formula injection in CSV export ([3916d53](https://github.com/julian-alarcon/DoTheSplit/commit/3916d5352278f5621243f13ab44bd034546aadd7))
* **api:** stop leaking raw database error from readyz probe ([a775be2](https://github.com/julian-alarcon/DoTheSplit/commit/a775be271effb53eebe406c6050e8220976589b3))
* **web:** apply baseline security headers site-wide, not just on admin ([16c5e51](https://github.com/julian-alarcon/DoTheSplit/commit/16c5e512f786a9a3e9054844576857ef131cc140))
* **web:** buttons in dark themes where not properly rendered, also admin ui was standarized ([e3ff57c](https://github.com/julian-alarcon/DoTheSplit/commit/e3ff57c7bdbcd70162456926084c198d56bbaef7))
* **web:** fix alignment error in recurrent expenses ([e115ae6](https://github.com/julian-alarcon/DoTheSplit/commit/e115ae6cc5f7ec29d3944f4e0517ad87159c9b3f))
* **web:** fix resources not being rendered with newer appraoch ([bc50106](https://github.com/julian-alarcon/DoTheSplit/commit/bc501061002d014082d825c395e22b14ba76c196))
* **web:** guard SSR forwarders against path traversal and add request timeout ([c2e1de0](https://github.com/julian-alarcon/DoTheSplit/commit/c2e1de05722398c11cd993ce7c6b1e2be2c47edc))
* **web:** improve showing the data and order of elements ([8c120fa](https://github.com/julian-alarcon/DoTheSplit/commit/8c120fa95d37f4e255fef9fd35e64785f6e2796c))
* **web:** improve view streching components and move elements to show more information ([b11adec](https://github.com/julian-alarcon/DoTheSplit/commit/b11adec10989263350f7b28590c52e33ba7da485))
* **web:** load more items when clicking the button ([88a14e8](https://github.com/julian-alarcon/DoTheSplit/commit/88a14e898bdd9ec31a7363e2e3457396433ac708))
* **web:** provide more vertical space in groups view changing the initial panel ([e6d07ee](https://github.com/julian-alarcon/DoTheSplit/commit/e6d07eea2cfe16a3b3937817e5aaacca351806f9))
* **web:** reject non-ascii upstream paths in SSR forwarder ([159ab62](https://github.com/julian-alarcon/DoTheSplit/commit/159ab62ae26051b5f72b34df841835b86548abbd))
* **web:** rename to high contrast ([abbd225](https://github.com/julian-alarcon/DoTheSplit/commit/abbd22566ac9131eef907e75d7b55f8fe0c8c099))
* **web:** show settlement payer/payee in activity log and restructure rows ([c0308c7](https://github.com/julian-alarcon/DoTheSplit/commit/c0308c7fa4eb68aca4b5fe7c52df26cfed83c7f7))
* **web:** surface API failures for expense/settlement/member actions ([a55fade](https://github.com/julian-alarcon/DoTheSplit/commit/a55fade0f9df1dbb4e9544c8c73136d717cb1dea))
* **web:** theme was reset temporary in high load calls ([e795109](https://github.com/julian-alarcon/DoTheSplit/commit/e79510921427e666d24318ae9c28493640666a88))
* **web:** truncate long names ([d823eaf](https://github.com/julian-alarcon/DoTheSplit/commit/d823eaf0c6aea49976d96133b718511bbb6b7d8b))

## [0.10.0](https://github.com/julian-alarcon/DoTheSplit/compare/v0.9.0...v0.10.0) (2026-06-07)


### Features

* **web,api:** import expenses in csv to existing group ([9258ad1](https://github.com/julian-alarcon/DoTheSplit/commit/9258ad1c28d1a67be85b52ef9f7591ed834ab9fa))
* **web:** added favicon ([63c9666](https://github.com/julian-alarcon/DoTheSplit/commit/63c9666f4e40e1b50d09b2c708a0f4c1c1cc75e8))

## [0.9.0](https://github.com/julian-alarcon/DoTheSplit/compare/v0.8.0...v0.9.0) (2026-06-06)


### Features

* **web,api:** add import from splitwise groups ([81f5039](https://github.com/julian-alarcon/DoTheSplit/commit/81f503972610e2289012d3423e2094ba23fd38c8))
* **web,api:** allow editing settlements and choosing payer when settling up ([631f7e8](https://github.com/julian-alarcon/DoTheSplit/commit/631f7e80615297e0b854e67cd2a6aa7f10e752f2))
* **web,api:** export and import groups via dothesplit CSV ([5d31e2f](https://github.com/julian-alarcon/DoTheSplit/commit/5d31e2f65559756cbabc432724972f3cccf1db0f))
* **web:** add brand logo to header, groups background, and README ([6579548](https://github.com/julian-alarcon/DoTheSplit/commit/6579548929d1c1a443133cd890ae5ec39dc7b360))

## [0.8.0](https://github.com/julian-alarcon/DoTheSplit/compare/v0.7.0...v0.8.0) (2026-06-03)


### Features

* **web,api:** added filters to search results ([f15f243](https://github.com/julian-alarcon/DoTheSplit/commit/f15f24355b8af697b725da1e8cabe97c470228da))
* **web,api:** added search feature ([b049c8d](https://github.com/julian-alarcon/DoTheSplit/commit/b049c8d653441441f7d44b437d2675d35fec2d7b))

## [0.7.0](https://github.com/julian-alarcon/DoTheSplit/compare/v0.6.0...v0.7.0) (2026-06-03)


### Features

* **web,api:** add password confirmation for delete user action, remove redundant login in settings ([c97f2f9](https://github.com/julian-alarcon/DoTheSplit/commit/c97f2f951c9501bee24a48cff5a5aeff7185f80c))


### Bug Fixes

* **web:** contrast highlight in user menu for dark and high contrast themes ([0699a10](https://github.com/julian-alarcon/DoTheSplit/commit/0699a1020ed248d7f9fe8df03a0398437f7543e4))
* **web:** contrast in switcher fixed in dark and high contrast themes ([26b95c7](https://github.com/julian-alarcon/DoTheSplit/commit/26b95c7a3baac809ae4c8a4952bb1d661d987a57))
* **web:** missing avatar border and size ([1ce1ea0](https://github.com/julian-alarcon/DoTheSplit/commit/1ce1ea073b1f1e70c0f0e1cd50709e20f7289fae))

## [0.6.0](https://github.com/julian-alarcon/DoTheSplit/compare/v0.5.1...v0.6.0) (2026-06-02)


### Features

* **web:** collapsible header user menu with search placeholder ([a3eaf40](https://github.com/julian-alarcon/DoTheSplit/commit/a3eaf407a8b6c0e681d02429b792f2b58ea4f3f8))

## [0.5.1](https://github.com/julian-alarcon/DoTheSplit/compare/v0.5.0...v0.5.1) (2026-06-02)


### Bug Fixes

* re added border for text input to improve visibility ([4db7e9e](https://github.com/julian-alarcon/DoTheSplit/commit/4db7e9e2beee04fd42622ba8010b98e3757ecc69))
* readded border for text input to improve visibility ([0852dcc](https://github.com/julian-alarcon/DoTheSplit/commit/0852dccbbc31035ffdcae911086e90b3f2edc8c5))
* updated credit files ([3c3945a](https://github.com/julian-alarcon/DoTheSplit/commit/3c3945a7e126adba10729617d3bb66c0dbeca480))

## [0.5.0](https://github.com/julian-alarcon/DoTheSplit/compare/v0.4.1...v0.5.0) (2026-06-02)


### Features

* added notes to expenses ([e0d6f74](https://github.com/julian-alarcon/DoTheSplit/commit/e0d6f749fd57e4c9d177ffad749e388a2732561c))
* added notes to expenses ([6a32e8b](https://github.com/julian-alarcon/DoTheSplit/commit/6a32e8b49c09f7733598f5cca5e8f04125c1d1a5))

## [0.4.1](https://github.com/julian-alarcon/DoTheSplit/compare/v0.4.0...v0.4.1) (2026-05-28)


### Bug Fixes

* **ci:** use PAT for release-please so tag pushes trigger downstream workflows ([6498426](https://github.com/julian-alarcon/DoTheSplit/commit/6498426220c571ff38750c8a86c5564e78379921))
* **ci:** use web/package.json as single version source ([0336003](https://github.com/julian-alarcon/DoTheSplit/commit/033600350da27c4acf47fe4bff1474dc573f8993))

## [0.4.0](https://github.com/julian-alarcon/dothesplit/compare/v0.3.0...v0.4.0) (2026-05-27)


### Features

* add categories and imnprove selection ([82a9dc8](https://github.com/julian-alarcon/dothesplit/commit/82a9dc86270001d3e5b7b592c242c87bc737444c))
* add footer with build info ([780113f](https://github.com/julian-alarcon/dothesplit/commit/780113f429dbe8987756ff5bb24ab10690225d85))
* add license attribution and CycloneDX SBOM compliance pipeline ([0edf4a2](https://github.com/julian-alarcon/dothesplit/commit/0edf4a26a810d73465936fd74cf1dac6da78139d))
* add recurrent expenses ([401a52d](https://github.com/julian-alarcon/dothesplit/commit/401a52d9a678566b96991f6f83f2432b8150965e))
* allow members management in groups (leave/remove) ([96873a4](https://github.com/julian-alarcon/dothesplit/commit/96873a4a25b633d512391bdf42071546e3afac27))
* **api,db:** email verification, plain-text mailer, change-email, notification prefs, admin SMTP send-test + reveal ([5c60003](https://github.com/julian-alarcon/dothesplit/commit/5c6000317c42c8938eca92179e4581eb1a118378))
* **api,db:** forgot/reset password by code, admin invite-by-email, drop must_change_password ([e9afb17](https://github.com/julian-alarcon/dothesplit/commit/e9afb17ca6e9f25e41ddc2fd0f80721f032479a0))
* **api,web,db:** admin role with users/groups/SMTP/audit and step-up password ([68ba7fa](https://github.com/julian-alarcon/dothesplit/commit/68ba7faec5854133f11184dd60a78474978324ba))
* **api,web:** settlements in activity feed with detail page ([f619839](https://github.com/julian-alarcon/dothesplit/commit/f619839c29a3c15bf1db00e5efcab1e6c115668a))
* **api:** add --healthcheck self-probe for distroless image ([1c3f3e8](https://github.com/julian-alarcon/dothesplit/commit/1c3f3e87d24a6cda04aeefbf03fd4371da061572))
* **api:** support *_FILE convention for secret env vars ([8229e96](https://github.com/julian-alarcon/dothesplit/commit/8229e9631af3043d650dc8869b7aaf0c20c4f076))
* automated release pipeline with GHCR publishing ([c402538](https://github.com/julian-alarcon/dothesplit/commit/c402538062171215496fa422b7f61a41f8ef1cc2))
* custom expenses splitting implemented ([998ff4b](https://github.com/julian-alarcon/dothesplit/commit/998ff4bde13362c292e3e37bf8279b301534da8c))
* **dev:** switch to AGENTS.md ([e998590](https://github.com/julian-alarcon/dothesplit/commit/e998590553d630646c17dc2e23e7bc50ca919350))
* **docker:** harden compose with read-only fs, dropped caps, healthchecks, loopback ports ([e25936d](https://github.com/julian-alarcon/dothesplit/commit/e25936d9b6360ad68dc3bb4f68eac375c47f99ae))
* **docs:** improved features information ([68bd513](https://github.com/julian-alarcon/dothesplit/commit/68bd513c092a238f9f7d0df2350a7088a9b11659))
* group default split percentage ([ff92825](https://github.com/julian-alarcon/dothesplit/commit/ff928250e243a85c1c059b788e1cd5b7922e23bf))
* increase image pixels to 8x8 ([31e1c0a](https://github.com/julian-alarcon/dothesplit/commit/31e1c0af17644f2691631f195fafa26e2e0ce3d9))
* set custom date of a expense ([914b9af](https://github.com/julian-alarcon/dothesplit/commit/914b9af5bd28cffe9537de1ad7345a746a1b74ee))
* transfer ownership ([e7eb5bf](https://github.com/julian-alarcon/dothesplit/commit/e7eb5bf19bc29455f8059a28595242b8cea05c87))
* **web,api,db:** introduce timezone management and setting ([88f9793](https://github.com/julian-alarcon/dothesplit/commit/88f979364cd4642d159674f2becffb9fe075e270))
* **web,api:** added pagination to the activity ([1412be0](https://github.com/julian-alarcon/dothesplit/commit/1412be0e1ea9a18005ddd928a71df04aed6d8650))
* **web,api:** allow more currencies ([ff1316c](https://github.com/julian-alarcon/dothesplit/commit/ff1316c0995edeb58c8e77794e3f4dd33a51782b))
* **web,api:** implement initial setup screen ([fea7f3a](https://github.com/julian-alarcon/dothesplit/commit/fea7f3a49c4b6dbaedcda08fb0666a40f9580693))
* **web:** add icons to destructive buttons and confirm dialog for leaving a group ([34be067](https://github.com/julian-alarcon/dothesplit/commit/34be0670522604b73af54ae6d3945ecaef55ad72))
* **web:** add theme switcher and externalize boot scripts for CSP ([058c9c2](https://github.com/julian-alarcon/dothesplit/commit/058c9c26da084b39bcc78bd687869c93054ac577))
* **web:** adopt new field/button system across pages and tighten UX ([7f28f46](https://github.com/julian-alarcon/dothesplit/commit/7f28f46add89d7cb8be85e7e2dcc0e6ba2dbaff4))
* **web:** allow adding members in a group creation ([e418f46](https://github.com/julian-alarcon/dothesplit/commit/e418f46078a04eae80d34b1e2573ce2bf55de1aa))
* **web:** currency picker flag glyphs and Palestine label override ([ff67d63](https://github.com/julian-alarcon/dothesplit/commit/ff67d639811cc3181e57476452b62dc3d81eafe8))
* **web:** email verification, change-email, notification prefs, admin SMTP password reveal + save-and-test ([b950f1d](https://github.com/julian-alarcon/dothesplit/commit/b950f1de45bd39abeaf30568d98ade5720912155))
* **web:** enlarge expense amounts with tabular-nums for readability ([425340c](https://github.com/julian-alarcon/dothesplit/commit/425340cbe97361974504801c0879335ee953ec04))
* **web:** floating-label fields, button utilities and shared icons ([a62c7df](https://github.com/julian-alarcon/dothesplit/commit/a62c7df679c43835dbed783c7e0d1af246686038))
* **web:** forgot/reset password pages, admin email-only invite, drop force-password-change ([43e2bee](https://github.com/julian-alarcon/dothesplit/commit/43e2beedbe0783aa606f008b13633a8d1bf385aa))
* **web:** render category and chrome icons via astro-icon (Font Awesome) ([495aafb](https://github.com/julian-alarcon/dothesplit/commit/495aafb0e58a004f4da6318dcd52e24876b69e10))
* **web:** reusable .toggle component for native checkbox switches ([f6d0f41](https://github.com/julian-alarcon/dothesplit/commit/f6d0f418d6f29c3743a40d1d79ad0b2b29f1f486))
* **web:** run as non-root and switch to npm ci ([4ceb16a](https://github.com/julian-alarcon/dothesplit/commit/4ceb16ab90fd6ab6a1e35a756d2c7fe119f61082))
* **web:** set better distribuition of elements in wide view ([1cb36e3](https://github.com/julian-alarcon/dothesplit/commit/1cb36e3ddc1229a6e034d35644664f475777ae0c))
* **web:** set inter font as default ([160f211](https://github.com/julian-alarcon/dothesplit/commit/160f211e168f5d6d1ae7a4bdca7847eb7dea7ed0))
* **web:** updated categories ([b0a13f1](https://github.com/julian-alarcon/dothesplit/commit/b0a13f1dee452daba0d4daa6dcc229b9e951ca97))


### Bug Fixes

* added ui fixes ([8c7bc4d](https://github.com/julian-alarcon/dothesplit/commit/8c7bc4d560055ae1adba93860fcd98d641cbff1f))
* **api,web:** allow any group member to delete expenses ([e556921](https://github.com/julian-alarcon/dothesplit/commit/e5569213431c2d2ca21fc28346d1e980a7f84b50))
* **api:** fix unordered list of activity (expenses/settlements) ([ac2e1a2](https://github.com/julian-alarcon/dothesplit/commit/ac2e1a27ea9554a19f4557c398031986f0edc7ee))
* **db:** missing cadence were added ([87fec23](https://github.com/julian-alarcon/dothesplit/commit/87fec230ee9efea51167212d3c5813e5305e5930))
* **web:** add expenses button now is height locked ([60409f7](https://github.com/julian-alarcon/dothesplit/commit/60409f7cce137b4d939085146e450b280366bd6d))
* **web:** buttons with same height ([97669b6](https://github.com/julian-alarcon/dothesplit/commit/97669b6b30edc0b0b2ea45787df3a3eee508a036))
* **web:** fix changing height of modal ([0a5024f](https://github.com/julian-alarcon/dothesplit/commit/0a5024f5a4cb09b99de1336f865352b622c365b7))
* **web:** fix position of group name ([bddc9e1](https://github.com/julian-alarcon/dothesplit/commit/bddc9e117c2c19b1c9073ecffa7f106c7c2f32a8))
* **web:** font color inconsistencies ([e44924a](https://github.com/julian-alarcon/dothesplit/commit/e44924a8a977c5b20ac1764e4d031e4e99cbad60))
* **web:** lock modals ([7ee9a1c](https://github.com/julian-alarcon/dothesplit/commit/7ee9a1c69b4f1d37f5e4652d880cc6d811a6b2ae))
* **web:** missing button in the top to go back ([a2c0db1](https://github.com/julian-alarcon/dothesplit/commit/a2c0db17dfd66807ae36e1839e8bf1f3ecb667f0))
* **web:** modal dialog show up moved scroll of page behind ([c75ff89](https://github.com/julian-alarcon/dothesplit/commit/c75ff89d48cad7fb7917d53c1a370756d0bb2171))
* **web:** my groups organization ([345c947](https://github.com/julian-alarcon/dothesplit/commit/345c9473579d8d45e02f81894b147bde0329a0bd))
* **web:** polish field, category trigger and header pill rendering ([95d89fc](https://github.com/julian-alarcon/dothesplit/commit/95d89fc5b2a7a9847abab7aafc458a25cf42d12d))
* **web:** removed pattern on amount ([c91ebce](https://github.com/julian-alarcon/dothesplit/commit/c91ebcee9d83aafd84b1d61eb75900773fe42553))
* **web:** rework category picker so trigger renders correctly in Chrome ([9f6741e](https://github.com/julian-alarcon/dothesplit/commit/9f6741e1ea5fb8675933a675a0b28fd426db55dc))
* **web:** set alternative alert when there are missing or invalid fields ([26ebbd2](https://github.com/julian-alarcon/dothesplit/commit/26ebbd259e2344d7b5ce358e525a0c11df157441))
* **web:** silence ts(6133) hint on admin user detail page ([d1a4eb3](https://github.com/julian-alarcon/dothesplit/commit/d1a4eb318e50cbacd12e39f2a16ffe5efcd7f301))
* **web:** split definition was not keep, and button was not enabled when editing after changing the split ([b0ba86f](https://github.com/julian-alarcon/dothesplit/commit/b0ba86fc11f160b3ffc7b22232dcb5e8e0c1f79f))
* **web:** wrap auth and new-group forms in a panel for input contrast ([6d1d4ee](https://github.com/julian-alarcon/dothesplit/commit/6d1d4ee2a60755c81e103c53675594ef8e7634e9))


### Performance Improvements

* **web:** drop unused Inter Italic/Bold variants (-470 KB preload) ([68a2d00](https://github.com/julian-alarcon/dothesplit/commit/68a2d0004dbe7bb3b609c428d2faf56217119326))
