from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

T = TypeVar("T", bound="TokenMetadata")


@_attrs_define
class TokenMetadata:
    """Enriched metadata for Stellar Asset Contracts (SAC) or tokens.

    Attributes:
        asset_code (str): Stellar asset code (e.g. USDC or native).
        issuer (str): Stellar asset issuer account ID (or empty for native).
        decimals (int): Number of decimal places (SAC deterministically uses 7).
        symbol (str): Token symbol.
        name (str): Token name or asset identifier.
    """

    asset_code: str
    issuer: str
    decimals: int
    symbol: str
    name: str
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        asset_code = self.asset_code

        issuer = self.issuer

        decimals = self.decimals

        symbol = self.symbol

        name = self.name

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "asset_code": asset_code,
                "issuer": issuer,
                "decimals": decimals,
                "symbol": symbol,
                "name": name,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        asset_code = d.pop("asset_code")

        issuer = d.pop("issuer")

        decimals = d.pop("decimals")

        symbol = d.pop("symbol")

        name = d.pop("name")

        token_metadata = cls(
            asset_code=asset_code,
            issuer=issuer,
            decimals=decimals,
            symbol=symbol,
            name=name,
        )

        token_metadata.additional_properties = d
        return token_metadata

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
