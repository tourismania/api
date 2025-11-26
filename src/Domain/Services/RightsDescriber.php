<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Domain\Services;

use App\Domain\Factory\RightsDescribeFactory;
use App\Domain\ValueObject\RightsDescribe;

readonly class RightsDescriber
{
    public function __construct(
        private RightsDescribeFactory $rightsDescribeFactory,
    ) {
    }

    /**
     * @param string[] $roles
     */
    public function byRoles(array $roles): RightsDescribe
    {
        return $this->rightsDescribeFactory->byRoles($roles);
    }
}
