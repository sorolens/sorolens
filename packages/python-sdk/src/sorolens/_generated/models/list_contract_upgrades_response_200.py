from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.contract_upgrade import ContractUpgrade


T = TypeVar("T", bound="ListContractUpgradesResponse200")


@_attrs_define
class ListContractUpgradesResponse200:
    """
    Attributes:
        upgrades (list[ContractUpgrade]):
    """

    upgrades: list[ContractUpgrade]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        upgrades = []
        for upgrades_item_data in self.upgrades:
            upgrades_item = upgrades_item_data.to_dict()
            upgrades.append(upgrades_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "upgrades": upgrades,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.contract_upgrade import ContractUpgrade

        d = dict(src_dict)
        upgrades = []
        _upgrades = d.pop("upgrades")
        for upgrades_item_data in _upgrades:
            upgrades_item = ContractUpgrade.from_dict(upgrades_item_data)

            upgrades.append(upgrades_item)

        list_contract_upgrades_response_200 = cls(
            upgrades=upgrades,
        )

        list_contract_upgrades_response_200.additional_properties = d
        return list_contract_upgrades_response_200

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
