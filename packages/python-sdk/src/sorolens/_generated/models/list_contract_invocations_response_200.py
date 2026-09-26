from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.invocation import Invocation


T = TypeVar("T", bound="ListContractInvocationsResponse200")


@_attrs_define
class ListContractInvocationsResponse200:
    """
    Attributes:
        invocations (list[Invocation]):
        next_cursor (str):
    """

    invocations: list[Invocation]
    next_cursor: str
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        invocations = []
        for invocations_item_data in self.invocations:
            invocations_item = invocations_item_data.to_dict()
            invocations.append(invocations_item)

        next_cursor = self.next_cursor

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "invocations": invocations,
                "next_cursor": next_cursor,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.invocation import Invocation

        d = dict(src_dict)
        invocations = []
        _invocations = d.pop("invocations")
        for invocations_item_data in _invocations:
            invocations_item = Invocation.from_dict(invocations_item_data)

            invocations.append(invocations_item)

        next_cursor = d.pop("next_cursor")

        list_contract_invocations_response_200 = cls(
            invocations=invocations,
            next_cursor=next_cursor,
        )

        list_contract_invocations_response_200.additional_properties = d
        return list_contract_invocations_response_200

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
