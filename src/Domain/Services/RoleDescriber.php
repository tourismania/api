<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Domain\Services;

use App\Domain\Factory\RoleDescribeFactory;
use App\Domain\ValueObject\RoleDescribe;

readonly class RoleDescriber
{
    public function __construct(
        private RoleDescribeFactory $roleDescribeFactory
    )
    {
    }

    /**
     * @param string[] $roles
     *
     * @return RoleDescribe
     */
    public function byRoles(array $roles): RoleDescribe
    {
        return $this->roleDescribeFactory->byRoles($roles);
    }

}
