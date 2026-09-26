from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.batch_contracts_request_action import BatchContractsRequestAction
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.batch_contracts_request_args import BatchContractsRequestArgs


T = TypeVar("T", bound="BatchContractsRequest")


@_attrs_define
class BatchContractsRequest:
    """
    Attributes:
        ids (list[str]): Contract IDs to act on (at most 100; duplicates are ignored).
        action (BatchContractsRequestAction): untrack permanently deletes the contracts and their indexed data; tag sets
            the label on each contract.
        args (BatchContractsRequestArgs | Unset):
    """

    ids: list[str]
    action: BatchContractsRequestAction
    args: BatchContractsRequestArgs | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        ids = self.ids

        action = self.action.value

        args: dict[str, Any] | Unset = UNSET
        if not isinstance(self.args, Unset):
            args = self.args.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "ids": ids,
                "action": action,
            }
        )
        if args is not UNSET:
            field_dict["args"] = args

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.batch_contracts_request_args import (
            BatchContractsRequestArgs,
        )

        d = dict(src_dict)
        ids = cast(list[str], d.pop("ids"))

        action = BatchContractsRequestAction(d.pop("action"))

        _args = d.pop("args", UNSET)
        args: BatchContractsRequestArgs | Unset
        if isinstance(_args, Unset):
            args = UNSET
        else:
            args = BatchContractsRequestArgs.from_dict(_args)

        batch_contracts_request = cls(
            ids=ids,
            action=action,
            args=args,
        )

        batch_contracts_request.additional_properties = d
        return batch_contracts_request

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
