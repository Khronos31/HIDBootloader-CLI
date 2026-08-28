# HIDBootloader CLI

Linux向けの、Microchip HID bootloader対応デバイス用コマンドライン書込みツールです。

初版の目的は、Bit Trade One AD00020/AD00020Pのように、出荷時にHID bootloaderが書き込まれたPIC18製品を、Qt GUIなしで更新できるようにすることです。

## 方針

本プロジェクトは、既存のMicrochipまたはBit Trade Oneのソースコード、実行ファイル、画像、ロゴを取り込みません。HIDデバイスアクセス、Intel HEXの解析、bootloader通信は独立した実装として作成します。

初版の対象環境はLinuxです。Qtや専用ランタイムを必要としない単一実行ファイルを配布できる構成を目指します。

## 状態

現在は仕様・ライセンス監査とプロジェクト骨格の段階です。実機に対する書込み機能は未完成です。

## ライセンス

このリポジトリで新規に作成したコードはMIT Licenseで提供します。第三者コンポーネントを追加する場合は、`NOTICE.md`と`docs/license-audit.md`を更新します。

Microchip、PIC、MCHPFSUSB、HID BootloaderおよびBit Trade Oneは、それぞれの権利者に帰属します。本プロジェクトは各社の公式製品ではありません。

ライセンス調査の記録は[`docs/license-audit.md`](docs/license-audit.md)にあります。

