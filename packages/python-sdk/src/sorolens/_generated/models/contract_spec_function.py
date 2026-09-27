from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.contract_spec_input import ContractSpecInput
    from ..models.contract_spec_type import ContractSpecType


T = TypeVar("T", bound="ContractSpecFunction")


@_attrs_define
class ContractSpecFunction:
    """
    Attributes:
        name (str):
        inputs (list[ContractSpecInput]):
        outputs (list[ContractSpecType]):
        doc (str | Unset):
    """

    name: str
    inputs: list[ContractSpecInput]
    outputs: list[ContractSpecType]
    doc: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        inputs = []
        for inputs_item_data in self.inputs:
            inputs_item = inputs_item_data.to_dict()
            inputs.append(inputs_item)

        outputs = []
        for outputs_item_data in self.outputs:
            outputs_item = outputs_item_data.to_dict()
            outputs.append(outputs_item)

        doc = self.doc

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "name": name,
                "inputs": inputs,
                "outputs": outputs,
            }
        )
        if doc is not UNSET:
            field_dict["doc"] = doc

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.contract_spec_input import ContractSpecInput
        from ..models.contract_spec_type import ContractSpecType

        d = dict(src_dict)
        name = d.pop("name")

        inputs = []
        _inputs = d.pop("inputs")
        for inputs_item_data in _inputs:
            inputs_item = ContractSpecInput.from_dict(inputs_item_data)

            inputs.append(inputs_item)

        outputs = []
        _outputs = d.pop("outputs")
        for outputs_item_data in _outputs:
            outputs_item = ContractSpecType.from_dict(outputs_item_data)

            outputs.append(outputs_item)

        doc = d.pop("doc", UNSET)

        contract_spec_function = cls(
            name=name,
            inputs=inputs,
            outputs=outputs,
            doc=doc,
        )

        contract_spec_function.additional_properties = d
        return contract_spec_function

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
