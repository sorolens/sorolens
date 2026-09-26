from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.search_result_type import SearchResultType
from ..types import UNSET, Unset

T = TypeVar("T", bound="SearchResult")


@_attrs_define
class SearchResult:
    """
    Attributes:
        type_ (SearchResultType):
        network (str):
        id (str | Unset): Contract ID; present for contract results.
        label (str | Unset): Contract label; present for contract results when set.
        contract_id (str | Unset): Contract associated with an event or invocation function result.
        tx_hash (str | Unset): Matching event transaction hash.
        function_name (str | Unset): Matching invocation function name.
    """

    type_: SearchResultType
    network: str
    id: str | Unset = UNSET
    label: str | Unset = UNSET
    contract_id: str | Unset = UNSET
    tx_hash: str | Unset = UNSET
    function_name: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        type_ = self.type_.value

        network = self.network

        id = self.id

        label = self.label

        contract_id = self.contract_id

        tx_hash = self.tx_hash

        function_name = self.function_name

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "type": type_,
                "network": network,
            }
        )
        if id is not UNSET:
            field_dict["id"] = id
        if label is not UNSET:
            field_dict["label"] = label
        if contract_id is not UNSET:
            field_dict["contract_id"] = contract_id
        if tx_hash is not UNSET:
            field_dict["tx_hash"] = tx_hash
        if function_name is not UNSET:
            field_dict["function_name"] = function_name

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        type_ = SearchResultType(d.pop("type"))

        network = d.pop("network")

        id = d.pop("id", UNSET)

        label = d.pop("label", UNSET)

        contract_id = d.pop("contract_id", UNSET)

        tx_hash = d.pop("tx_hash", UNSET)

        function_name = d.pop("function_name", UNSET)

        search_result = cls(
            type_=type_,
            network=network,
            id=id,
            label=label,
            contract_id=contract_id,
            tx_hash=tx_hash,
            function_name=function_name,
        )

        search_result.additional_properties = d
        return search_result

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
