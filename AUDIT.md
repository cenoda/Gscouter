# Gscouter `main` 감사 보고

감사 대상은 `main` (`3338fe71`)이다. 프로덕션 코드는 바꾸지 않았다. `TestRepro_`로 시작하는 테스트는 아래 결함을 재현하며, 현재 코드에서는 실패하는 것이 맞다.

## 요약

| 심각도 | 건수 |
|---|---|
| critical | 0 |
| high | 0 |
| medium | 3 |
| low | 6 |

`go test -race -covermode=atomic -coverprofile ./...`는 통과했고, 데이터 레이스는 없었다. 문장 기준 전체 커버리지는 **70.5%**이다. `go test`가 실행하는 `go vet`도 통과했다. 설치된 `staticcheck`는 Go 1.26으로 빌드되어 이 모듈(Go 1.27)을 분석하지 못했다.

패키지별 커버리지 중 위험이 큰 빈칸은 다음과 같다.

| 패키지 | 커버리지 | 비고 |
|---|---|---|
| `internal/web` | 90.6% | 높지만 취소·동시 조회·빈 쿼리는 비어 있다 |
| `internal/chart` | 94.5% | 정수 오버플로는 없다 |
| `internal/share` | 54.7% | HTTP 클라이언트와 세미콜론이 빠진 설정은 없다 |
| `internal/score` | 77.3% | |
| `internal/steady` | 76.7% | |
| `internal/validate` | 7.8% | 타임아웃·날짜 경로가 거의 없다 |
| `internal/kqm` | 0% | 패키지 전체 |
| `internal/batch` | 31.0% | `Isolated`는 이 패키지 테스트에서 0% |
| `cmd/server`, `cmd/import`, `cmd/validate` | 0% | `main`만 있다 |
| `cmd/weights` | 3.8% | |
| `cmd/compare` | 8.0% | |

기존 테스트가 강한 부분(라이브러리 로테이션, 차트 문법의 정상 경로, Enka 변환 픽스처, 적 파라미터의 정상·범위 밖 값)은 레이스 디텍터 아래에서도 깨지지 않았다. 아래 항목은 그 테스트가 보지 않는 분기이다.

## Medium

### 1. 취소된 저장 빌드 조회가 1분 동안 재사용된다

- 심각도: medium
- 위치: `internal/web/handlers.go` `EnkaSource.SavedBuilds` (833–844행), 캐시 TTL `savedRetry` (827–829행)

`SavedBuilds`는 Enka가 거절한 응답을 `savedRetry`(1분) 동안 캐시한다. `TestSavedBuildsCached`는 HTTP 500이 한 번만 나가도록 이 동작을 고정해 두었다. 다만 호출자의 `context`가 취소되거나 핸들러의 10초 제한(`addSaved`)에 걸려도 같은 분기에 들어간다. `FetchBuilds`가 돌려주는 오류는 `context.Canceled` / `context.DeadlineExceeded`를 감싼 값이고, 그 값이 소유자(`enka.Owner`) 키로 프로세스 전역 캐시에 남는다.

다음 방문자의 컨텍스트가 정상이어도 캐시가 그 오류를 그대로 돌려준다. 쇼케이스 자체는 뜨지만, 그 계정의 저장 빌드는 1분 동안 아무도 보지 못한다. 인기 UID에서 한 번의 타임아웃이 다른 방문자에게 퍼진다.

기존 테스트는 취소 없이 500만 본다. 재현은 `TestRepro_SavedBuildsDoesNotCacheContextCancel`이다. 첫 요청을 핸들러에 들어간 뒤 취소하면 Enka 호출은 1번이고, 새 `context.Background()`로 다시 호출해도 `context canceled`가 반환된다.

수정 방향: `context.Canceled`와 `context.DeadlineExceeded`는 캐시하지 않는다. Enka가 실제로 거절한 상태 코드만 `savedRetry`로 기억한다.

### 2. 같은 UID의 동시 조회가 Enka로 그대로 나간다

- 심각도: medium
- 위치: `internal/web/handlers.go` `EnkaSource.Profile` (793–815행)

캐시는 응답이 돌아온 뒤에만 채워진다. 그 전에 같은 UID로 들어온 요청은 각자 `Fetch`를 호출한다. 설계(`docs/design.md` §4.1)는 Enka 호출 제한을 지키고 응답을 TTL 동안 재사용하라고 한다. 프런트와 링크 미리보기 카드가 같은 UID를 동시에 열면, 캐시가 비어 있는 동안 호출이 방문자 수만큼 나간다. 429가 나면 그 조회는 실패하고, 실패는 캐시되지 않으므로 다음 요청이 다시 나간다.

기존 테스트는 조회를 한 번씩 순서로 호출한다. 재현은 `TestRepro_EnkaProfileSingleflight`이다. 핸들러가 응답을 붙잡고 있는 동안 같은 UID를 두 번 조회하면 Enka에 2번 도달한다.

수정 방향: UID별 singleflight(진행 중인 요청에 합류)를 둔다. `SavedBuilds`도 같은 형태가 안전하다.

### 3. 빈 `res`가 모든 내성을 0%로 만든다

- 심각도: medium
- 위치: `internal/web/enemy.go` `enemyFromQuery` (55–82행)

`read`는 값이 비어 있으면 성공으로 보고 대상 정수를 바꾸지 않는다. `lvl`이나 `pyro`에 빈 값이 오면 그래서 기본값이 유지된다. `res`만 `url.Values.Has`가 참인 뒤 `var all int`(0)을 쓰고, `read`가 빈 문자열에서 참을 반환하면 그 0을 여덟 원소 내성 전부에 복사한다.

`?res=`와 `?res=%20`은 따라서 Lv.100, 내성 `[0 0 0 0 0 0 0 0]`인 커스텀 적으로 점수에 들어간다. 표준 적은 내성 10%이다. 0%는 범위(-100–1000) 안이라 `Validate`도 통과한다. 점수 차이가 크고, 페이지는 형식이 잘못됐다고 하지 않은 채 커스텀 적으로 계산한다.

`TestEnemyQuery`는 `res=10`, `res=25`, `res=2000`만 본다. 빈 값은 없다. 재현은 `TestRepro_EmptyResIsRejected`이다.

수정 방향: `res`가 있는데 공백을 지운 값이 비어 있거나 정수가 아니면 `err.enemy`를 반환한다. 빈 `lvl`과 같이 “파라미터 없음”으로 둘 것이면 `Has`만으로 적용하지 않는다.

## Low

### 4. 사용량 집계 고루틴이 멈추지 않는다

- 심각도: low
- 위치: `internal/web/usage.go` `SetUsage` (61–67행). `rand.Read` 오류는 `day` (143행)에서 버려진다.

`SetUsage`는 `time.Tick(10 * time.Minute)` 루프를 고루틴으로 띄운다. `Tick`의 채널은 소비자를 멈출 수 없고, `FlushUsage`도 그 고루틴을 종료하지 않는다. 서버 프로세스가 살아있는 동안에는 고루틴이 하나라 운영상 티는 작다. 테스트는 `SetUsage`를 여러 번 부르고(`TestUsage`가 두 번) 프로세스가 끝날 때까지 고루틴이 남는다. `go test`는 고루틴 누수를 실패로 보지 않는다.

하루가 바뀌면 `rand.Read`로 키를 다시 뽑는데 오류를 무시한다. 실패하면 전날 키가 그대로다. 해시 자체는 디스크에 쓰지 않으므로 UID가 파일로 새지는 않는다. 다만 키를 바꾸려다 실패한 사실이 로그에도 남지 않는다.

재현은 `TestRepro_FlushUsageStopsBackgroundFlush`이다. `FlushUsage` 이후에도 `SetUsage.func1` 스택이 하나 남는다.

수정 방향: `time.NewTicker`와 `context`로 종료하게 하고, 서버 종료 시 그 컨텍스트를 취소한 뒤 `FlushUsage`를 호출한다. `rand.Read` 오류는 반환하거나 로그한다.

### 5. 차트 대기 합이 `int`를 넘어 한도를 통과한다

- 심각도: low
- 위치: `internal/chart/chart.go` `Parse` (308–322행). 지연(`+`)도 같은 덧셈이다.

`~` 토큰은 `strconv.Atoi`만 통과하면 `wait += v`를 한다. 합이 `MaxWait`(3600프레임)을 넘으면 `Validate`가 거절하지만, 덧셈 자체는 오버플로를 검사하지 않는다. `MaxInt + MaxInt + 100`은 2의 보수에서 98이 되고, 98은 한도 안이다. `Validate`는 그 차트를 받는다. 본문이 `MaxText`(8000)를 넘지 않으므로 서비스 거부로 이어지지는 않는다. 적은 대기 값이 적힌 것과 다른 의미로 통과하는 것이 문제다. `+`는 `isDigits`로 부호를 막지만 덧셈 오버플로는 같다.

차트 테스트는 한도 안의 대기와 음수 하나짜리를 본다. 재현은 `TestRepro_WaitSumCannotWrapIntoRange`이다. 위 입력의 대기 값은 98프레임으로 파싱된다.

수정 방향: 더하기 전에 `wait > MaxWait-v`(v가 양수일 때)를 검사하거나, `int64`로 모아 `MaxWait`를 넘으면 그 자리에서 오류를 반환한다.

### 6. 세미콜론이 없는 설정 줄이 다음 문장을 삼킨다

- 심각도: low
- 위치: `internal/share/share.go` `SplitConfig` (156–186행)

gcsim이 받아들이는 설정은 문장이 세미콜론으로 끝난다. 그런데 이 분할기는 세미콜론이 없으면 `inSetup`을 유지한 채 다음 줄을 같은 문장에 붙인다. 아래 입력에서 `active nahida;`는 빌드 줄에 흡수되고 로테이션에서는 사라진다.

```
nahida char lvl=90/90 cons=6 talent=1,1,1
active nahida;
nahida skill;
```

가져오기(`cmd/import`)는 이 함수로 빌드와 로테이션을 가른다. 세미콜론을 빠뜨린 공유 설정을 넣으면 오류 대신 액티브가 없는 로테이션이 저장된다. 이후 검증이 실패하거나, 실패하지 않으면 잘못된 로테이션이 남는다.

`internal/share` 테스트는 세미콜론이 갖춰진 픽스처만 쓴다. 네트워크 쪽 `Fetch`/`resolve`/`get`은 커버리지에 없다(패키지 54.7%). 재현은 `TestRepro_SplitConfigDoesNotSwallowNextStatement`이다.

수정 방향: 파일 끝이나 다음 문장이 시작됐는데 세미콜론이 없으면 오류를 반환한다. 빌드에 `active`가 들어가면 실패로 본다.

### 7. 커뮤니티 DB·공유 설정 클라이언트가 응답 크기와 자체 시간 제한이 없다

- 심각도: low
- 위치: `internal/kqm/kqm.go` `fetchAll` (149–160행), `fetch` (163–191행). `internal/share/share.go` `get` (89–103행), `resolve`의 DB 리다이렉트(67–76행)

두 클라이언트 모두 `http.DefaultClient`를 쓴다. 시간 제한은 호출자가 넘긴 `context`뿐이다. `cmd/harvest`는 5분, `cmd/import`는 30초를 걸므로 그 진입점에서는 무한 대기가 잘린다. 패키지를 `context.Background()`로 부르면 서버가 연결만 받고 본문을 주지 않을 때 반환되지 않는다. 본문은 `io.ReadAll`이라 크기 상한이 없다. Enka 클라이언트(`internal/enka/client.go`)는 15초 시간 제한과 10MiB `LimitReader`가 있다. 이쪽에는 없다.

`fetchAll`은 페이지가 `pageSize`(100)보다 짧아질 때까지 `skip`을 더한다. 서버가 `skip`을 무시하고 항상 100건을 주면 루프가 끝나지 않는다. 호출자 컨텍스트가 있어야만 멈춘다. 같은 ID가 반복되는지도 보지 않는다.

`internal/kqm`는 테스트가 없다(0%). `share` 테스트는 로컬 문자열 분할만 한다.

수정 방향: 두 패키지에 시간 제한이 있는 `http.Client`와 본문 상한을 둔다. `fetchAll`은 이미 본 ID가 다시 오거나 페이지가 더 늘지 않으면 멈춘다.

### 8. 검증 기록 날짜가 로컬 시간대다

- 심각도: low
- 위치: `internal/validate/validate.go` `Validation` (162행)

```go
Date: time.Now().Format(time.DateOnly),
```

사용량 집계와 KQM `Updated`는 UTC다. 이 날짜만 프로세스 로컬 시간대다. 이 환경(UTC)에서는 차이가 없어 단위 테스트가 실패하지 않는다. `TZ=Pacific/Niue`이면 같은 순간이 UTC 2026-10-31, 로컬 2026-10-30이다. `cmd/validate`가 `team.yaml`에 적는 날짜가 하루 어긋날 수 있다. `internal/validate` 커버리지는 7.8%이고 이 줄의 시간대를 고정해 둔 테스트는 없다.

수정 방향: `time.Now().UTC().Format(time.DateOnly)`.

### 9. `ListenAndServe` 실패 시 `os.Exit`가 정리를 건너뛴다

- 심각도: low
- 위치: `cmd/server/main.go` (107–110행). `FlushUsage`는 111–113행, 링크 DB `defer store.Close()`는 62행

`ListenAndServe`가 `http.ErrServerClosed`가 아닌 오류를 반환하면 `os.Exit(1)`이다. `os.Exit`는 defer를 실행하지 않으므로 `store.Close()`와 `FlushUsage`가 빠진다. 기동 직후 포트 충돌이면 아직 집계가 더럽지 않아 손실은 작다. 수신이 시작된 뒤 같은 경로로 빠지면 마지막 10분의 사용량 카운트가 디스크에 없다. `cmd/server` 커버리지는 0%이다.

수정 방향: `os.Exit` 전에 `FlushUsage`와 `store.Close`를 호출하거나, 오류를 반환하고 `main` 끝에서 정리한 뒤 종료 코드를 정한다.

## 테스트가 약하거나 비어 있는 곳

아래는 위의 결함과 겹치지만, 테스트가 “있다”는 사실만으로 가려지는 부분이다.

- `TestSavedBuildsCached`는 오류 캐시를 의도적으로 잠근다. HTTP 500과 호출자 취소를 구분하지 않는다. 이번 1번 결함이 그 틈이다.
- `TestEnemyQuery`의 거절 목록은 `lvl=0`, `lvl=x`, `res=2000`, `pyro=-101`, `geo=1.5`, `boss=nobody`이다. 빈 `res=`는 없다.
- `TestUsage`는 시계를 주입해 날짜 변경과 재시작 후 카운트는 확인한다. `SetUsage`가 만든 고루틴은 세지 않는다. UID를 재시작 뒤에 다시 세면 하루 키가 메모리에만 있어 두 번 세어지는 것은 주석으로 문서화되어 있고, 테스트는 그 경우를 단언하지 않는다.
- `internal/kqm` 전체, `cmd/server`의 종료 경로, `internal/batch.Isolated`(`batch.go` 137–154행, 이 패키지 테스트에서 0%)는 실행되지 않는다. `Isolated`는 자식의 종료 코드(`runErr`)를 stdout JSON이 파싱되면 버린다. 자식이 보고서를 쓴 뒤 0이 아닌 코드로 끝나면 부모는 성공으로 본다. `Serve`가 정상 경로에서 `os.Exit(0)`을 하므로 지금 명령에서는 바로 터지지는 않는다.
- `internal/validate` 7.8%에는 `probe`(451–479행)의 20초 타임아웃이 없다. 타임아웃은 `time.After`라 부모 컨텍스트를 그 20초에 맞추지 않고, 시뮬레이션 고루틴은 프로세스가 끝날 때까지 돈다. `cmd/validate`와 `cmd/harvest`는 `CheckIsolated`로 자식 프로세스를 쓰고 자식이 `os.Exit`하므로 운영 경로에서는 프로세스가 곧 죽는다. in-process로 `Check`를 부르면 그 고루틴이 남는다.
- 레이트 리밋 키는 `internal/web/short.go` `clientIP`(192–201행)가 `X-Forwarded-For`의 첫 항목을 믿는다. 주석은 Caddy가 그 헤더를 교체한다고 한다. 현재 Caddy `reverse_proxy`는 신뢰하지 않는 피어의 `X-Forwarded-For`를 클라이언트 주소로 바꾸고, 저장소의 `deploy/Caddyfile`은 `trusted_proxies`를 두지 않는다. 지금 배포 조합에서는 위조가 통하지 않는다. 앱 자체는 프록시를 검사하지 않고, 그 가정을 잠그는 테스트도 없다. 앞에 헤더를 이어 붙이는 프록시를 두면 짧은 링크·전송 제한이 우회된다.
- 링크 미리보기 카드의 `fetchImage`(`internal/web/card.go` 380–403행)는 아이콘 URL의 본문 크기를 제한하지 않는다. URL은 게임 데이터에서 만든 Enka 주소라 방문자가 호스트를 고르지는 못한다.
- 전체 레이스 테스트가 통과했으므로, 이번 감사에서 확인한 문제는 데이터 레이스가 아니라 위 분기들이다. `score`의 `stillGuard`와 틱 콜백은 기존 시뮬레이션 테스트의 레이스 디텍터를 통과했다.
