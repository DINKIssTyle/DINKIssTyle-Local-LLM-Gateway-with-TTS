# 운영체제별 트레이 아이콘

각 운영체제 폴더의 SVG 또는 실제 빌드 파일을 직접 편집할 수 있습니다.

| 운영체제 | 편집 원본 | 앱이 사용하는 파일 |
| --- | --- | --- |
| macOS | `build/darwin/trayicon.svg` | `build/darwin/trayicon.png` (36×36 px, 18×18 pt @2x) |
| Linux | `build/linux/trayicon.svg` | `build/linux/trayicon.png` (64×64 px) |
| Windows | `build/windows/trayicon.svg` | `build/windows/trayicon.ico` (16~256 px 다중 해상도) |

PNG 또는 ICO를 직접 편집했다면 그대로 빌드하면 됩니다. SVG를 편집했다면 다음
명령으로 실제 빌드 파일을 다시 생성합니다.

```sh
./scripts/generate-tray-icons.sh
```

특정 운영체제만 생성할 수도 있습니다.

```sh
./scripts/generate-tray-icons.sh darwin
./scripts/generate-tray-icons.sh linux
./scripts/generate-tray-icons.sh windows
```

macOS 메뉴바 아이콘은 투명 배경의 단색 템플릿으로 유지해야 밝은/어두운 메뉴바에
자동으로 맞춰집니다. 빌드 스크립트는 직접 편집한 PNG/ICO를 덮어쓰지 않으며 파일의
존재 여부만 확인합니다.
