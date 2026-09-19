# ADR-0006: Policy övertrumfar användaroverride

## Status

Accepterad

## Kontext

Användare kan vilja sätta modell explicit, men team- och säkerhetspolicy måste kunna blockera otillåtna val.

## Beslut

Override respekteras endast inom policygränser. Policy kan tvinga, blockera eller höja modellnivå.

## Konsekvenser

Säkerheten blir starkare. Vissa användare kan uppleva att routern inte gör vad de bad om, därför behövs tydliga explanations.

## Alternativ

Låt användarval alltid vinna; blockera endast via providerkeys.
