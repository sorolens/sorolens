from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.ttl_operation import TTLOperation


T = TypeVar("T", bound="ListContractTtlHistoryResponse200")


@_attrs_define
class ListContractTtlHistoryResponse200:
    """
    Attributes:
        operations (list[TTLOperation]):
        next_cursor (str): Opaque cursor for the next (older) page; empty when no more pages.
    """

    operations: list[TTLOperation]
    next_cursor: str
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        operations = []
        for operations_item_data in self.operations:
            operations_item = operations_item_data.to_dict()
            operations.append(operations_item)

        next_cursor = self.next_cursor

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "operations": operations,
                "next_cursor": next_cursor,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.ttl_operation import TTLOperation

        d = dict(src_dict)
        operations = []
        _operations = d.pop("operations")
        for operations_item_data in _operations:
            operations_item = TTLOperation.from_dict(operations_item_data)

            operations.append(operations_item)

        next_cursor = d.pop("next_cursor")

        list_contract_ttl_history_response_200 = cls(
            operations=operations,
            next_cursor=next_cursor,
        )

        list_contract_ttl_history_response_200.additional_properties = d
        return list_contract_ttl_history_response_200

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
