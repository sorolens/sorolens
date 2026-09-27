from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.batch_contracts_result_action import BatchContractsResultAction

T = TypeVar("T", bound="BatchContractsResult")


@_attrs_define
class BatchContractsResult:
    """
    Attributes:
        action (BatchContractsResultAction):
        requested (int): Number of distinct contract IDs in the request.
        affected (int): Number of contracts actually changed (unknown IDs are ignored).
    """

    action: BatchContractsResultAction
    requested: int
    affected: int
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        action = self.action.value

        requested = self.requested

        affected = self.affected

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "action": action,
                "requested": requested,
                "affected": affected,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        action = BatchContractsResultAction(d.pop("action"))

        requested = d.pop("requested")

        affected = d.pop("affected")

        batch_contracts_result = cls(
            action=action,
            requested=requested,
            affected=affected,
        )

        batch_contracts_result.additional_properties = d
        return batch_contracts_result

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
